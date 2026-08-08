package logging

import (
	"bytes"
	"log/slog"
	"testing"
)

func TestNormalizeLogArgs_KeyValuePairs(t *testing.T) {
	got := normalizeLogArgs("stderr", "boom", "context", "lint failed")
	want := []any{"stderr", "boom", "context", "lint failed"}
	assertAnySlice(t, got, want)
}

func TestNormalizeLogArgs_SlogAttr(t *testing.T) {
	got := normalizeLogArgs(slog.String("op", "close"), "path", "/tmp/x")
	want := []any{"op", "close", "path", "/tmp/x"}
	assertAnySlice(t, got, want)
}

func TestNormalizeLogArgs_DanglingKeyDropped(t *testing.T) {
	got := normalizeLogArgs("a", 1, "orphan")
	want := []any{"a", 1}
	assertAnySlice(t, got, want)
}

func TestReportError_KeyValuePairs(t *testing.T) {
	var buf bytes.Buffer
	h := NewPlainHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	ctx := NewRootContext(slog.New(h))

	if !ReportError(ctx, errSentinel{}, "context", "unit test") {
		t.Fatal("expected ReportError to report non-nil err")
	}
	out := buf.String()
	if out == "" {
		t.Fatal("expected log output")
	}
	// Plain handler emits key=value; just sanity-check message and attrs land.
	for _, sub := range []string{"unexpected error", "context", "unit test", "error"} {
		if !bytes.Contains([]byte(out), []byte(sub)) {
			t.Errorf("log output missing %q: %q", sub, out)
		}
	}
}

func TestReportError_NilErr(t *testing.T) {
	ctx := NewRootContext(slog.Default())
	if ReportError(ctx, nil, "context", "should not log") {
		t.Fatal("expected false for nil err")
	}
}

type errSentinel struct{}

func (errSentinel) Error() string { return "sentinel" }

func assertAnySlice(t *testing.T, got, want []any) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len got=%d want=%d\ngot=%v\nwant=%v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("idx %d: got %#v want %#v", i, got[i], want[i])
		}
	}
}
