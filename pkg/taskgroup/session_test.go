package taskgroup

import (
	"context"
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
	s.AfterWait(func() { after.Store(true) })
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
