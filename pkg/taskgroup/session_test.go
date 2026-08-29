package taskgroup

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

func TestSessionCloseWaitsAndRunsAfterWait(t *testing.T) {
	s, ctx := Enter(withLogger(t), DefaultLimits())
	var ran atomic.Bool
	var after atomic.Bool
	g := MustFromContext(ctx)
	g.Go("work", CPU, func(ctx context.Context, st *Status) error {
		ran.Store(true)
		return nil
	})
	s.AfterWait(func() error { after.Store(true); return nil })
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !ran.Load() || !after.Load() {
		t.Fatalf("ran=%v after=%v", ran.Load(), after.Load())
	}
	// Idempotent
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestSessionFromContext(t *testing.T) {
	s, ctx := Enter(withLogger(t), DefaultLimits())
	if SessionFrom(ctx) != s {
		t.Fatal("SessionFrom mismatch")
	}
	if MustSessionFrom(ctx) != s {
		t.Fatal("MustSessionFrom mismatch")
	}
	_ = s.Close()
}

func TestSessionAfterWaitError(t *testing.T) {
	s, ctx := Enter(withLogger(t), DefaultLimits())
	g := MustFromContext(ctx)
	g.Go("ok", CPU, func(ctx context.Context, st *Status) error { return nil })
	s.AfterWait(func() error { return errors.New("hook fail") })
	err := s.Close()
	if err == nil || err.Error() != "hook fail" {
		t.Fatalf("got %v", err)
	}
}

func TestSessionLazyUINoGo(t *testing.T) {
	// Without Go(), UI must not start even when wantUI would be true on a tty.
	s, _ := Enter(withLogger(t), DefaultLimits())
	s.wantUI = true
	if s.prog != nil {
		t.Fatal("UI started at Enter without Go")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if s.prog != nil {
		t.Fatal("UI started during Close without Go")
	}
}

func TestSessionOnScheduleWired(t *testing.T) {
	// First Go invokes onSchedule (lazy UI hook) without requiring a real TUI.
	s, ctx := Enter(withLogger(t), DefaultLimits())
	var n atomic.Int32
	s.group.onSchedule = func() { n.Add(1) }
	// Also wire through SubGroup path used by Map/lint.
	child, _ := s.group.SubGroup(ctx)
	child.Go("t", CPU, func(ctx context.Context, st *Status) error { return nil })
	if n.Load() < 1 {
		t.Fatal("expected onSchedule from SubGroup.Go")
	}
	_ = s.Close()
}
