package taskgroup

import (
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

func TestTeaWriterCloseNoErrAfterPipeTraffic(t *testing.T) {
	// Mirrors session teardown after a chatty home apply: many lines through
	// the stderr pipe bridge, then restore/close. Must not return
	// "read |0: file already closed".
	var n atomicCounter
	w := &teaWriter{print: func(string) { n.Add(1) }}
	f, err := w.File()
	if err != nil {
		t.Fatal(err)
	}

	const writers = 8
	const lines = 100
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < lines; j++ {
				if _, err := fmt.Fprintf(f, "stderr %d-%d\n", id, j); err != nil {
					return
				}
				if _, err := w.Write([]byte(fmt.Sprintf("direct %d-%d\n", id, j))); err != nil {
					return
				}
			}
		}(i)
	}
	wg.Wait()

	// Partial line left in the bridge (dropped on close by design).
	if _, err := io.WriteString(f, "partial"); err != nil {
		t.Fatal(err)
	}

	if err := w.close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if n.Load() == 0 {
		t.Fatal("expected some printed lines")
	}
}

func TestTeaWriterCloseIdempotentWithoutFile(t *testing.T) {
	w := &teaWriter{print: func(string) {}}
	if err := w.close(); err != nil {
		t.Fatalf("close without File: %v", err)
	}
	if err := w.close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestTeaWriterCloseAfterSlowPrint(t *testing.T) {
	// print holds mu via Write; close must wait, then exit cleanly (no hang,
	// no closed-pipe error). Timeout is provided by go test -timeout.
	w := &teaWriter{print: func(string) {
		time.Sleep(5 * time.Millisecond)
	}}
	f, err := w.File()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if _, err := fmt.Fprintf(f, "line %d\n", i); err != nil {
			t.Fatal(err)
		}
	}
	// Give the copy goroutine a head start into Write/print.
	time.Sleep(2 * time.Millisecond)
	if err := w.close(); err != nil {
		t.Fatalf("close after slow print: %v", err)
	}
}

type atomicCounter struct {
	mu sync.Mutex
	n  int
}

func (c *atomicCounter) Add(d int) {
	c.mu.Lock()
	c.n += d
	c.mu.Unlock()
}

func (c *atomicCounter) Load() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}
