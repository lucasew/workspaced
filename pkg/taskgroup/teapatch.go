package taskgroup

import (
	"bufio"
	"bytes"
	"log"
	"log/slog"
	"os"
	"sync"

	"workspaced/pkg/logging"

	tea "charm.land/bubbletea/v2"
)

// teaLineWriter is an io.Writer that line-buffers incoming bytes and, for each
// complete line (terminated by '\n'), invokes the provided print func with the
// line content (without the delimiter). Partial final lines are held until a
// terminating newline arrives or flush is called.
//
// This is used to route raw stderr writes and default slog records through
// prog.Printf while a bubbletea program owns the terminal, so that
// third-party libraries using the stdlib logger, slog.Default(), or direct
// writes to os.Stderr still appear in the TUI transcript above the bars.
type teaLineWriter struct {
	print func(string)
	mu    sync.Mutex
	buf   []byte
}

func (w *teaLineWriter) Write(p []byte) (int, error) {
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

// flush emits any buffered partial line (no trailing newline). Called on
// restore so the last line from a library that forgot the \n is not lost.
func (w *teaLineWriter) flush() {
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
// It does three things, only while the returned cleanup has not run:
//   - Monkey-patches os.Stderr (via an os.Pipe whose read end is scanned
//     and each line fed to prog.Printf + refreshMsg). This catches direct
//     writes from sub-libraries and exec.Cmds that inherited the var.
//   - Replaces slog.Default() with a logger whose handler writes formatted
//     records through a teaLineWriter -> prog.Printf. This catches any
//     code doing slog.SetDefault or just using the package default.
//   - Replaces the output of the stdlib log.Default() the same way.
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

	// Line printer used by both the stderr pipe pump and the teaLineWriter.
	// We Printf the content (and Send refresh for the onLog path).
	// For the catch-all pump/tlw path we intentionally omit Send(refreshMsg{}):
	// prog.Send after Quit can block the caller (the pump goroutine), which
	// would hang the <-pumpDone in cleanupPatches and thus prevent return
	// from RunBubbleTea / the command after the model has already decided to
	// quit. The 100ms tick loop will refresh bars on its own schedule.
	printLine := func(s string) {
		prog.Printf("%s", s)
		// Only the primary task onLog handler (see RunBubbleTea) does the
		// Printf+Send combination for "log line just appeared, push bars down".
		// The patches path skips Send to keep shutdown (pump drain) robust.
	}

	// Set up a pipe so that writes to the os.Stderr package var after this
	// point are routed into a scanner that emits via printLine.
	pr, pw, perr := os.Pipe()
	if perr != nil {
		// Pipe failed (very rare); proceed with slog/log patches only.
		pr = nil
		pw = nil
	} else {
		os.Stderr = pw
	}

	// Pump goroutine: read lines (and final unterminated line) from the pipe
	// read end and forward them into the tea program.
	pumpDone := make(chan struct{})
	if pr != nil {
		go func() {
			defer close(pumpDone)
			sc := bufio.NewScanner(pr)
			for sc.Scan() {
				printLine(sc.Text())
			}
		}()
	} else {
		close(pumpDone)
	}

	// teaLineWriter is the io.Writer used by the slog handler (and stdlib log).
	// Each complete line it extracts is sent via printLine (printf+refresh).
	tlw := &teaLineWriter{print: printLine}

	// Install a new default slog whose records render with our project's
	// compact formatter and ultimately land in the tea program.
	h := logging.NewPlainHandler(tlw, &slog.HandlerOptions{
		Level: slog.LevelDebug, // capture everything third parties emit
	})
	slog.SetDefault(slog.New(h))

	// Also capture the stdlib "log" package output.
	log.SetOutput(tlw)

	// The cleanup func is the "exit" of the context-manager.
	return func() {
		// Stop new log records from being produced into our writers first.
		slog.SetDefault(oldSlog)
		log.SetOutput(oldLogWriter)

		// Close the pipe write end so the scanner goroutine sees EOF and exits.
		if pw != nil {
			pw.Close()
		}
		if pr != nil {
			// Eagerly close the read side too. This guarantees the bufio.Scanner
			// in the pump sees EOF and exits even if a concurrent printLine
			// (or other) would otherwise slow it. We still wait for the goroutine
			// to close pumpDone for cleanliness, but the double close makes
			// hangs in the drain much less likely.
			pr.Close()
			<-pumpDone
		}

		// Restore the os.Stderr var for any code that reads it after us.
		os.Stderr = oldStderr

		// Best-effort: emit a trailing partial line that had no '\n'.
		tlw.flush()
	}
}
