package taskgroup

import (
	"io"
	"strings"
	"testing"
)

func TestLineWriterLazyUntilVisible(t *testing.T) {
	hub := newLiveHub()
	w := newLineWriter(hub, func(string) {})
	if _, err := w.Write([]byte("\x1b[2K\r")); err != nil {
		t.Fatal(err)
	}
	if rows := hub.snapshot(); len(rows) != 0 {
		t.Fatalf("snapshot = %q, want empty until visible text", rows)
	}
	if _, err := w.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if rows := hub.snapshot(); len(rows) != 1 || rows[0] != "hello" {
		t.Fatalf("snapshot = %q, want [hello]", rows)
	}
}

func TestLineWriterCommitOnNewline(t *testing.T) {
	hub := newLiveHub()
	var committed []string
	w := newLineWriter(hub, func(s string) { committed = append(committed, s) })
	if _, err := w.Write([]byte("10%\r100%\ndone\n")); err != nil {
		t.Fatal(err)
	}
	if strings.Join(committed, "|") != "100%|done" {
		t.Fatalf("committed = %q", committed)
	}
	if rows := hub.snapshot(); len(rows) != 0 {
		t.Fatalf("live rows after commit = %q", rows)
	}
}

func TestLineWriterCloseCommitsLeftover(t *testing.T) {
	hub := newLiveHub()
	var committed []string
	w := newLineWriter(hub, func(s string) { committed = append(committed, s) })
	if _, err := w.Write([]byte("almost")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(committed, "|") != "almost" {
		t.Fatalf("committed = %q, want [almost]", committed)
	}
	if rows := hub.snapshot(); len(rows) != 0 {
		t.Fatalf("live rows after close = %q", rows)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestLineWriterReadFromCloses(t *testing.T) {
	hub := newLiveHub()
	var committed []string
	w := newLineWriter(hub, func(s string) { committed = append(committed, s) })
	n, err := w.ReadFrom(strings.NewReader("10%\r100%"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 8 {
		t.Fatalf("ReadFrom n = %d, want 8", n)
	}
	if strings.Join(committed, "|") != "100%" {
		t.Fatalf("committed = %q, want [100%%]", committed)
	}
}

func TestLineWriterTwoIndependentRows(t *testing.T) {
	hub := newLiveHub()
	a := newLineWriter(hub, func(string) {})
	b := newLineWriter(hub, func(string) {})
	if _, err := a.Write([]byte("alpha")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write([]byte("beta")); err != nil {
		t.Fatal(err)
	}
	rows := hub.snapshot()
	if len(rows) != 2 || rows[0] != "alpha" || rows[1] != "beta" {
		t.Fatalf("snapshot = %q, want [alpha beta]", rows)
	}
	if _, err := a.Write([]byte("\n")); err != nil {
		t.Fatal(err)
	}
	rows = hub.snapshot()
	if len(rows) != 1 || rows[0] != "beta" {
		t.Fatalf("after a commit: %q, want [beta]", rows)
	}
}

func TestLiveHubAbandonAll(t *testing.T) {
	hub := newLiveHub()
	var printed []string
	w := newLineWriter(hub, func(s string) { printed = append(printed, s) })
	if _, err := w.Write([]byte("leftover")); err != nil {
		t.Fatal(err)
	}
	got := hub.abandonAll()
	if strings.Join(got, "|") != "leftover" {
		t.Fatalf("abandon = %q", got)
	}
	if len(printed) != 0 {
		t.Fatalf("abandon must not print, got %q", printed)
	}
	if _, err := w.Write([]byte("x")); err != io.ErrClosedPipe {
		t.Fatalf("write after abandon: err = %v", err)
	}
}

func TestLineWriterFromNoSession(t *testing.T) {
	w := LineWriterFrom(t.Context())
	lw, ok := w.(*lineWriter)
	if !ok {
		t.Fatalf("got %T, want *lineWriter", w)
	}
	if lw.commitOnClose {
		t.Fatal("no-session writer must not flush leftover on close")
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFinishedLineWriterOnlyEmitsCompleteLines(t *testing.T) {
	var buf strings.Builder
	w := newFinishedLineWriter(&buf)
	if _, err := w.Write([]byte("10%\r50%\r100%\ndone\npartial")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "100%\ndone\n" {
		t.Fatalf("got %q, want only finished lines", got)
	}
}
