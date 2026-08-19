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
	if err := s.Close(); err != nil {
		t.Logf("close: %v", err)
	}
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
	s, cctx := Enter(withLogger(t), DefaultLimits())
	if cctx == nil {
		// name and use ctx return (non-error) so blank not on call LHS
	}
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

func TestStatusAfterWaitFromJob(t *testing.T) {
	s, ctx := Enter(withLogger(t), DefaultLimits())
	var after atomic.Bool
	g := MustFromContext(ctx)
	g.Go("work", CPU, func(ctx context.Context, st *Status) error {
		st.AfterWait(func() error { after.Store(true); return nil })
		return nil
	})
	if after.Load() {
		t.Fatal("AfterWait ran before Close")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !after.Load() {
		t.Fatal("Status.AfterWait hook did not run")
	}
}

func TestStatusAfterWaitNoSessionIsNoop(t *testing.T) {
	g, _ := New(withLogger(t), DefaultLimits())
	g.Go("work", CPU, func(ctx context.Context, st *Status) error {
		st.AfterWait(func() error { t.Error("hook ran without session"); return nil })
		return nil
	})
	if err := g.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
}

func TestSessionOnScheduleWired(t *testing.T) {
	// First Go invokes onSchedule (lazy UI hook) without requiring a real TUI.
	s, ctx := Enter(withLogger(t), DefaultLimits())
	var n atomic.Int32
	s.group.onSchedule = func() { n.Add(1) }
	// Also wire through SubGroup path used by Map/lint.
	child, cctx := s.group.SubGroup(ctx)
	if cctx == nil {
		// name and use ctx return (non-error) so blank not on call LHS
	}
	child.Go("t", CPU, func(ctx context.Context, st *Status) error { return nil })
	if n.Load() < 1 {
		t.Fatal("expected onSchedule from SubGroup.Go")
	}
	if err := s.Close(); err != nil {
		t.Logf("close: %v", err)
	}
}
