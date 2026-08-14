package taskgroup

import (
	"context"
	"io"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
)

// liveHub holds the in-progress (not yet newline-terminated) row for each
// LineWriter. The bubbletea model snapshots this every tick.
type liveHub struct {
	mu      sync.Mutex
	seq     atomic.Uint64
	order   []string
	slots   map[string]string
	writers map[string]*lineWriter
}

func newLiveHub() *liveHub {
	return &liveHub{
		slots:   make(map[string]string),
		writers: make(map[string]*lineWriter),
	}
}

func (h *liveHub) snapshot() []string {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, 0, len(h.order))
	for _, id := range h.order {
		if text, ok := h.slots[id]; ok && text != "" {
			out = append(out, text)
		}
	}
	return out
}

func (h *liveHub) set(id, text string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if text == "" {
		h.dropLocked(id)
		return
	}
	if _, ok := h.slots[id]; !ok {
		h.order = append(h.order, id)
	}
	h.slots[id] = text
}

func (h *liveHub) drop(id string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dropLocked(id)
}

func (h *liveHub) dropLocked(id string) {
	if _, ok := h.slots[id]; !ok {
		return
	}
	delete(h.slots, id)
	for i, x := range h.order {
		if x == id {
			h.order = append(h.order[:i], h.order[i+1:]...)
			break
		}
	}
}

func (h *liveHub) register(w *lineWriter) {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.writers[w.id] = w
	h.mu.Unlock()
}

// abandonAll marks every writer closed and returns leftover live text.
// Does not print — caller writes leftovers to the restored stderr.
func (h *liveHub) abandonAll() []string {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	ws := make([]*lineWriter, 0, len(h.writers))
	for _, w := range h.writers {
		ws = append(ws, w)
	}
	h.writers = make(map[string]*lineWriter)
	h.slots = make(map[string]string)
	h.order = nil
	h.mu.Unlock()

	var leftover []string
	for _, w := range ws {
		if s := w.abandon(); s != "" {
			leftover = append(leftover, s)
		}
	}
	return leftover
}

// lineWriter is one stream's live row. First visible glyph registers the
// row; '\n' commits it via print; Close commits any leftover.
type lineWriter struct {
	id    string
	hub   *liveHub
	print func(string)

	mu     sync.Mutex
	filter lineFilter
	closed bool
}

func newLineWriter(hub *liveHub, print func(string)) *lineWriter {
	w := &lineWriter{
		hub:   hub,
		print: print,
	}
	if hub != nil {
		w.id = "live-" + strconv.FormatUint(hub.seq.Add(1), 10)
		hub.register(w)
	}
	return w
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	committed := w.filter.write(p)
	cur := w.filter.current()
	print := w.print
	id := w.id
	hub := w.hub
	w.mu.Unlock()

	for _, line := range committed {
		if print != nil {
			print(line)
		}
	}
	hub.set(id, cur)
	return len(p), nil
}

// ReadFrom is used by os/exec when Stderr is not an *os.File: the copy
// ends when the child closes its pipe, which means the process is done
// writing — commit leftover then.
func (w *lineWriter) ReadFrom(r io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var n int64
	for {
		nr, err := r.Read(buf)
		if nr > 0 {
			nw, werr := w.Write(buf[:nr])
			n += int64(nw)
			if werr != nil {
				_ = w.Close()
				return n, werr
			}
		}
		if err != nil {
			cErr := w.Close()
			if err == io.EOF {
				return n, cErr
			}
			if cErr != nil {
				return n, cErr
			}
			return n, err
		}
	}
}

func (w *lineWriter) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	text := w.filter.take()
	print := w.print
	id := w.id
	hub := w.hub
	w.mu.Unlock()

	hub.drop(id)
	if text != "" && print != nil {
		print(text)
	}
	return nil
}

func (w *lineWriter) abandon() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return ""
	}
	w.closed = true
	w.print = nil
	return w.filter.take()
}

// LineWriterFrom returns a writer for one subprocess stream.
//
// With an active session UI the writer is a live row: CR / erase-line rewrite
// that row until a newline commits it above the bars. Close commits leftover.
// Without a session it is a passthrough to os.Stderr (Close is a no-op).
func LineWriterFrom(ctx context.Context) io.WriteCloser {
	if s := SessionFrom(ctx); s != nil {
		return s.LineWriter()
	}
	return passWriter{os.Stderr}
}

// LineWriter returns a new live-row writer for this session. The row appears
// only after the first visible character. Safe to assign to cmd.Stdout and
// cmd.Stderr (same writer for both).
func (s *Session) LineWriter() io.WriteCloser {
	if s == nil || !s.wantUI {
		return passWriter{os.Stderr}
	}
	s.ensureUI()
	if s.live == nil {
		return passWriter{os.Stderr}
	}
	return newLineWriter(s.live, s.commitLine)
}

func (s *Session) commitLine(msg string) {
	if s == nil || s.prog == nil || msg == "" {
		return
	}
	s.prog.Printf("%s", msg)
}

// passWriter writes through and ignores Close (must not close os.Stderr).
type passWriter struct{ w io.Writer }

func (p passWriter) Write(b []byte) (int, error) { return p.w.Write(b) }
func (p passWriter) Close() error                { return nil }
