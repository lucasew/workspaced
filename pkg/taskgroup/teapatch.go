package taskgroup

import (
	"bytes"
	"io"
	"log"
	"log/slog"
	"os"
	"sync"

	"workspaced/pkg/logging"

	tea "charm.land/bubbletea/v2"
)

// teaWriter is the single line-buffering front door for TUI transcript output
// while bubbletea owns the terminal. For each complete line (terminated by
// '\n') it invokes print with the line content (without the delimiter).
// Partial final lines are held until a terminating newline arrives or close.
//
// Direct io.Writer users (slog, stdlib log) call Write. Callers that need an
// *os.File (os.Stderr, exec inheritance) use File(), which opens a pipe and
// runs go io.Copy(w, readEnd) so all bytes still pass through Write.
type teaWriter struct {
	print func(string)

	mu  sync.Mutex
	buf []byte

	// Pipe backing File(); nil until File() succeeds.
	pipeR, pipeW *os.File
	copyDone     chan struct{}
}

func (w *teaWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := bytes.TrimSuffix(w.buf[:i], []byte{'\r'})
		w.buf = w.buf[i+1:]
		if w.print != nil {
			w.print(string(line))
		}
	}
	return len(p), nil
}

// File returns an *os.File whose writes are bridged into w via a background
// io.Copy. The same File is returned on every successful call. Call close
// (via the installTeaPatches cleanup) to tear the pipe down and flush.
//
// The background copy is started without holding mu so Write can lock.
func (w *teaWriter) File() (*os.File, error) {
	w.mu.Lock()
	if w.pipeW != nil {
		f := w.pipeW
		w.mu.Unlock()
		return f, nil
	}
	w.mu.Unlock()

	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(w, pr)
	}()

	w.mu.Lock()
	if w.pipeW != nil {
		// Lost a race; keep the winner and discard this pipe.
		f := w.pipeW
		w.mu.Unlock()
		_ = pw.Close()
		_ = pr.Close()
		<-done
		return f, nil
	}
	w.pipeR, w.pipeW, w.copyDone = pr, pw, done
	w.mu.Unlock()
	return pw, nil
}

// close tears down the optional File() pipe, waits for the background copy,
// and emits any trailing partial line. Safe to call when File() was never used.
func (w *teaWriter) close() {
	w.mu.Lock()
	pw, pr, done := w.pipeW, w.pipeR, w.copyDone
	w.pipeW, w.pipeR, w.copyDone = nil, nil, nil
	w.mu.Unlock()

	if pw != nil {
		_ = pw.Close()
	}
	if pr != nil {
		// Unblock Copy if stuck in a slow Write, then wait for it.
		_ = pr.Close()
	}
	if done != nil {
		<-done
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) == 0 {
		return
	}
	if w.print != nil {
		w.print(string(w.buf))
	}
	w.buf = nil
}

// installTeaPatches applies (and returns a cleanup that restores) global
// output redirections so that code unaware of our context-logger or
// taskgroup onLog path still has its output appear inside the active
// bubbletea program.
//
// All patched sources converge on one teaWriter: slog/log write to it
// directly; os.Stderr is reassigned to tw.File() (*os.File for exec).
//
// The style is deliberately "Python context manager": save originals,
// install, return a func that puts everything back. Call the func with defer.
//
// Patching only happens for the interactive RunBubbleTea path; the
// non-tty early return in RunBubbleTea never installs anything.
func installTeaPatches(prog *tea.Program) (cleanup func()) {
	oldStderr := os.Stderr
	oldSlog := slog.Default()
	oldLogWriter := log.Default().Writer()

	// Printf only — no Send(refreshMsg{}). prog.Send after Quit can block the
	// background io.Copy writer path and stall cleanup. Bars refresh via the
	// 100ms tick; task onLog in RunBubbleTea still does Printf+Send.
	tw := &teaWriter{print: func(s string) { prog.Printf("%s", s) }}

	if f, err := tw.File(); err == nil {
		os.Stderr = f
	}

	h := logging.NewPlainHandler(tw, &slog.HandlerOptions{
		Level: slog.LevelDebug, // capture everything third parties emit
	})
	slog.SetDefault(slog.New(h))
	log.SetOutput(tw)

	return func() {
		slog.SetDefault(oldSlog)
		log.SetOutput(oldLogWriter)
		os.Stderr = oldStderr
		tw.close()
	}
}
