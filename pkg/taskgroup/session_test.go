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
