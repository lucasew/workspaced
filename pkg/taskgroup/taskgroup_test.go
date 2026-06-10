package taskgroup

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestBasicExecution(t *testing.T) {
	g, ctx := New(context.Background(), DefaultLimits())
	var ran atomic.Bool
	g.Go("task1", CPU, func(ctx context.Context, s *Status) error {
		ran.Store(true)
		return nil
	})
	if err := g.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ran.Load() {
		t.Fatal("task did not run")
	}
	_ = ctx
}

func TestDependencyOrder(t *testing.T) {
	g, _ := New(context.Background(), DefaultLimits())
	order := make([]string, 0, 3)
	var mu atomic.Int64

	g.Go("a", CPU, func(ctx context.Context, s *Status) error {
		time.Sleep(10 * time.Millisecond)
		mu.Add(1)
		return nil
	})
	g.Go("b", CPU, func(ctx context.Context, s *Status) error {
		if mu.Load() < 1 {
			t.Error("b ran before a")
		}
		return nil
	}, "a")
	g.Go("c", CPU, func(ctx context.Context, s *Status) error {
		return nil
	}, "a", "b")
	if err := g.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = order
}

func TestErrorCancelsGroup(t *testing.T) {
	g, _ := New(context.Background(), DefaultLimits())
	sentinel := errors.New("boom")

	g.Go("fail", CPU, func(ctx context.Context, s *Status) error {
		return sentinel
	})
	g.Go("after", CPU, func(ctx context.Context, s *Status) error {
		t.Error("should not run after dep failure")
		return nil
	}, "fail")

	err := g.Wait()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPoolLimits(t *testing.T) {
	limits := Limits{IO: 2, CPU: 2, Internet: 2}
	g, _ := New(context.Background(), limits)

	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	for i := 0; i < 10; i++ {
		name := "t" + string(rune('0'+i))
		g.Go(name, IO, func(ctx context.Context, s *Status) error {
			cur := concurrent.Add(1)
			for {
				old := maxConcurrent.Load()
				if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			concurrent.Add(-1)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maxConcurrent.Load() > 2 {
		t.Fatalf("max concurrent %d exceeded pool limit 2", maxConcurrent.Load())
	}
}

func TestSnapshot(t *testing.T) {
	g, _ := New(context.Background(), DefaultLimits())

	g.Go("x", CPU, func(ctx context.Context, s *Status) error {
		s.Update("working")
		s.Progress(50, 100)
		s.Log("did a thing")
		return nil
	})
	if err := g.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	snap := g.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("expected 1 task, got %d", len(snap))
	}
	if snap[0].State != Done {
		t.Fatalf("expected Done, got %s", snap[0].State)
	}
}

func TestSubGroup(t *testing.T) {
	// Pool size must be >= 2: parent holds one CPU slot, child needs another.
	g, ctx := New(context.Background(), Limits{IO: 2, CPU: 2, Internet: 2})
	g.Go("parent", CPU, func(ctx context.Context, s *Status) error {
		child, childCtx := g.SubGroup(ctx)
		child.Go("child1", CPU, func(ctx context.Context, s *Status) error {
			return nil
		})
		_ = childCtx
		return child.Wait()
	})
	if err := g.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = ctx
}

func TestFromContext(t *testing.T) {
	g, ctx := New(context.Background(), DefaultLimits())
	got := FromContext(ctx)
	if got != g {
		t.Fatal("FromContext did not return the group")
	}
	if FromContext(context.Background()) != nil {
		t.Fatal("FromContext on empty context should return nil")
	}
}
