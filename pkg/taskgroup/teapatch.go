package taskgroup

import (
	"bytes"
	"io"
	"os"
	"sync"
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
// (via outputEnv.restore) to tear the pipe down and flush.
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
// and drops any trailing partial line. Safe to call when File() was never used.
//
// Intentionally does not call print/Printf: session teardown may run while the
// bubbletea event loop is already wedged or about to Quit; flushing via
// Program.Printf here deadlocks ("all goroutines are asleep") under
// WORKSPACED_FORCE_TUI or short-lived sessions. Transcript loss of a partial
// final line is preferable to hanging the process.
func (w *teaWriter) close() {
	w.mu.Lock()
	// Stop accepting print callbacks before unblocking the pipe copy so a
	// concurrent Write from io.Copy cannot re-enter Program.Printf.
	w.print = nil
	w.buf = nil
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
}
