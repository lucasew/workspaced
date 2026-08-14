package taskgroup

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
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
	// raw, when set, receives every byte unchanged (used to feed a lineWriter
	// so CR / CSI stay intact). print is ignored in that mode.
	raw io.Writer

	mu  sync.Mutex
	buf []byte

	// Pipe backing File(); nil until File() succeeds.
	pipeR, pipeW *os.File
	copyDone     chan struct{}
	copyErr      error
}

func (w *teaWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	raw := w.raw
	w.mu.Unlock()
	if raw != nil {
		return raw.Write(p)
	}

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
		if _, err := io.Copy(w, pr); err != nil {
			w.mu.Lock()
			w.copyErr = err
			w.mu.Unlock()
		}
	}()

	w.mu.Lock()
	if w.pipeW != nil {
		// Lost a race; keep the winner and discard this pipe.
		f := w.pipeW
		w.mu.Unlock()
		var errs []error
		if err := pw.Close(); err != nil {
			errs = append(errs, err)
		}
		if err := pr.Close(); err != nil {
			errs = append(errs, err)
		}
		<-done
		if len(errs) > 0 {
			// Prefer the winning pipe; report race cleanup via copyErr for close().
			w.mu.Lock()
			for _, e := range errs {
				w.copyErr = e
			}
			w.mu.Unlock()
		}
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
//
// Order matters: nil print (so Write cannot block on tea), close the write end
// (EOF for io.Copy), wait for the copy goroutine, then close the read end.
// Closing the read end while Copy is still reading races to
// "read |0: file already closed" and was incorrectly failing home apply after
// a successful plan/execute.
func (w *teaWriter) close() error {
	w.mu.Lock()
	// Stop accepting print callbacks before unblocking the pipe copy so a
	// concurrent Write from io.Copy cannot re-enter Program.Printf.
	// Write holds mu across print, so this wait also drains any in-flight
	// Printf; afterward Write is non-blocking (print is nil).
	// Keep raw until the copy finishes so leftover CR bytes still reach the
	// lineWriter; abandonAll then commits them onto the restored stderr.
	w.print = nil
	w.buf = nil
	pw, pr, done := w.pipeW, w.pipeR, w.copyDone
	w.pipeW, w.pipeR, w.copyDone = nil, nil, nil
	w.mu.Unlock()

	var errs []error
	if pw != nil {
		if err := pw.Close(); err != nil && !isBenignPipeCloseErr(err) {
			errs = append(errs, err)
		}
	}
	// Wait for EOF-driven Copy exit before closing pr. Closing pr first made
	// io.Copy observe fs.ErrClosed and surface it as a session failure.
	if done != nil {
		<-done
	}
	w.mu.Lock()
	w.raw = nil
	w.mu.Unlock()
	if pr != nil {
		if err := pr.Close(); err != nil && !isBenignPipeCloseErr(err) {
			errs = append(errs, err)
		}
	}
	w.mu.Lock()
	if w.copyErr != nil && !isBenignPipeCloseErr(w.copyErr) {
		errs = append(errs, w.copyErr)
	}
	w.copyErr = nil
	w.mu.Unlock()
	return errors.Join(errs...)
}

// isBenignPipeCloseErr reports teardown races that are not real failures:
// closing a pipe end while the peer copy is finishing.
func isBenignPipeCloseErr(err error) bool {
	return errors.Is(err, fs.ErrClosed) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, os.ErrClosed)
}
