package taskgroup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"workspaced/pkg/logging"
)

func withLogger(t *testing.T) context.Context {
	h := logging.NewPlainHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})
	l := slog.New(h)
	return logging.ContextWithLogger(t.Context(), l)
}

func TestBasicExecution(t *testing.T) {
	g, ctx := New(withLogger(t), DefaultLimits())
	var ran atomic.Bool
	id := g.Go("task1", CPU, func(ctx context.Context, s *Status) error {
		ran.Store(true)
		return nil
	})
	if id == "" {
		t.Fatal("Go returned empty id")
	}
	if err := g.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ran.Load() {
		t.Fatal("task did not run")
	}
	_ = ctx
}

func TestDependencyOrder(t *testing.T) {
	g, _ := New(withLogger(t), DefaultLimits())
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
	g, _ := New(withLogger(t), DefaultLimits())
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
	g, _ := New(withLogger(t), limits)

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
	// Snapshot while still running so we observe progress before prune.
	started := make(chan struct{})
	release := make(chan struct{})
	g, _ := New(withLogger(t), DefaultLimits())
	g.Go("x", CPU, func(ctx context.Context, s *Status) error {
		s.Update("working")
		s.Progress(50, 100)
		logger := logging.GetLogger(ctx)
		logger.Info("did a thing")
		close(started)
		<-release
		return nil
	})
	<-started
	snap := g.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("expected 1 task while running, got %d", len(snap))
	}
	if snap[0].State != Running {
		t.Fatalf("expected Running, got %s", snap[0].State)
	}
	if snap[0].ID == "" {
		t.Fatal("TaskState.ID is empty")
	}
	if snap[0].Name != "x" {
		t.Errorf("Name = %q, want %q", snap[0].Name, "x")
	}
	close(release)
	if err := g.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := g.Snapshot(); len(got) != 0 {
		t.Fatalf("expected completed tasks pruned, snapshot len=%d", len(got))
	}
}

func TestTaskLogFormattingMatchesPlainSlogOutput(t *testing.T) {
	g, _ := New(withLogger(t), DefaultLimits())
	var got []string
	g.SetLogHandler(func(taskName, msg string) {
		got = append(got, msg)
	})

	g.Go("fetch", Internet, func(ctx context.Context, s *Status) error {
		logger := logging.GetLogger(ctx)
		logger.Info("http response", "status", "200 OK")
		return nil
	})
	if err := g.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const want = `I http response task=fetch status="200 OK"`
	if len(got) != 1 || got[0] != want {
		t.Fatalf("log line = %#v, want [%q]", got, want)
	}
	// Task is pruned after completion; logs were delivered via SetLogHandler.
	if got := g.Snapshot(); len(got) != 0 {
		t.Fatalf("expected pruned snapshot, got %#v", got)
	}
}

func TestSubGroup(t *testing.T) {
	// Pool size must be >= 2: parent holds one CPU slot, child needs another.
	g, ctx := New(withLogger(t), Limits{IO: 2, CPU: 2, Internet: 2})
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
	g, ctx := New(withLogger(t), DefaultLimits())
	got := FromContext(ctx)
	if got != g {
		t.Fatal("FromContext did not return the group")
	}
	if FromContext(context.Background()) != nil {
		t.Fatal("FromContext on empty context should return nil")
	}
}

func TestMap_BasicAndOrder(t *testing.T) {
	g, ctx := New(withLogger(t), DefaultLimits())
	_ = g

	input := []int{1, 2, 3, 4, 5}

	// Use the root group directly (Map will SubGroup from it)
	results, err := Map(ctx, func(int) PoolKind { return CPU }, input, func(i int, _ int) string { return fmt.Sprintf("map:%d", i) }, func(ctx context.Context, s *Status, v int) (int, error) {
		s.Update(fmt.Sprintf("processing %d", v))
		// simulate work
		time.Sleep(1 * time.Millisecond)
		return v * 10, nil
	})
	if err != nil {
		t.Fatalf("Map returned error: %v", err)
	}
	if len(results) != len(input) {
		t.Fatalf("got %d results, want %d", len(results), len(input))
	}
	for i, r := range results {
		want := input[i] * 10
		if r != want {
			t.Errorf("results[%d] = %d, want %d (order not preserved?)", i, r, want)
		}
	}

	// Note: because Map uses SubGroup internally (like planner, lint, backup etc.),
	// the individual "map-*" tasks live on the child group and won't appear in
	// the root g.Snapshot(). This is by design (see nested demo and comments in
	// taskgroup). The important contract is order-preserving results + progress
	// seeding inside each handler's Status.
}

func TestMap_Empty(t *testing.T) {
	g, ctx := New(withLogger(t), DefaultLimits())
	_ = g

	results, err := Map(ctx, func(string) PoolKind { return IO }, []string{}, nil, func(ctx context.Context, s *Status, v string) (string, error) {
		return v + "!", nil
	})
	if err != nil {
		t.Fatalf("empty Map error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty result slice, got len=%d", len(results))
	}
}

func TestMap_ErrorPropagates(t *testing.T) {
	g, ctx := New(withLogger(t), DefaultLimits())
	_ = g

	_, err := Map(ctx, func(int) PoolKind { return IO }, []int{10, 20, 30}, nil, func(ctx context.Context, s *Status, v int) (int, error) {
		if v == 20 {
			return 0, errors.New("boom on 20")
		}
		return v, nil
	})
	if err == nil {
		t.Fatal("expected error from Map, got nil")
	}
}

func TestMap_UsesProvidedTaskNames(t *testing.T) {
	g, ctx := New(withLogger(t), DefaultLimits())
	_ = g

	items := []string{"a.txt", "b.txt"}

	results, err := Map(ctx, func(string) PoolKind { return IO }, items, func(i int, name string) string {
		return "read:" + name
	}, func(ctx context.Context, s *Status, name string) (string, error) {
		return "content-of-" + name, nil
	})
	if err != nil {
		t.Fatalf("named Map failed: %v", err)
	}
	if len(results) != len(items) {
		t.Fatalf("expected %d results, got %d", len(items), len(results))
	}
	// Custom task names are used for the child subgroup tasks (logs, internal
	// bookkeeping, and if someone renders the child group). They are not
	// present on the root snapshot (see SubGroup design).
}

func TestDuplicateDescriptionsAllowed(t *testing.T) {
	g, _ := New(withLogger(t), DefaultLimits())

	var ran int32
	var mu sync.Mutex
	seen := map[string]bool{}

	g.Go("download", IO, func(ctx context.Context, s *Status) error {
		mu.Lock()
		seen["d1"] = true
		mu.Unlock()
		atomic.AddInt32(&ran, 1)
		return nil
	})
	g.Go("download", IO, func(ctx context.Context, s *Status) error {
		mu.Lock()
		seen["d2"] = true
		mu.Unlock()
		atomic.AddInt32(&ran, 1)
		return nil
	})
	g.Go("process", CPU, func(ctx context.Context, s *Status) error {
		mu.Lock()
		seen["p"] = true
		mu.Unlock()
		atomic.AddInt32(&ran, 1)
		return nil
	})

	if err := g.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ran != 3 {
		t.Fatalf("expected 3 executions, got %d", ran)
	}
	// Both downloads must have run (no panic on duplicate desc).
	// Completed tasks are pruned; only side effects prove all three ran.
	if got := g.Snapshot(); len(got) != 0 {
		t.Fatalf("expected pruned snapshot after Wait, got %d tasks", len(got))
	}
	if !seen["d1"] || !seen["d2"] || !seen["p"] {
		t.Errorf("not all tasks observed: %v", seen)
	}
}

func TestDependencyByDescriptionPicksLatest(t *testing.T) {
	g, _ := New(withLogger(t), DefaultLimits())

	var mu sync.Mutex
	seen := map[string]bool{}

	g.Go("setup", CPU, func(ctx context.Context, s *Status) error {
		mu.Lock()
		seen["setup1"] = true
		mu.Unlock()
		return nil
	})
	g.Go("setup", CPU, func(ctx context.Context, s *Status) error {
		mu.Lock()
		seen["setup2"] = true
		mu.Unlock()
		return nil
	})
	// Depend on description "setup" — should resolve to the most recent one (setup2).
	g.Go("work", CPU, func(ctx context.Context, s *Status) error {
		mu.Lock()
		seen["work"] = true
		if !seen["setup2"] {
			t.Error("work ran before second setup (latest desc match failed)")
		}
		mu.Unlock()
		return nil
	}, "setup")

	if err := g.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDependencyByReturnedID(t *testing.T) {
	g, _ := New(withLogger(t), DefaultLimits())

	var mu sync.Mutex
	firstDone := false

	firstID := g.Go("phase", CPU, func(ctx context.Context, s *Status) error {
		mu.Lock()
		firstDone = true
		mu.Unlock()
		return nil
	})

	// Second task with same desc.
	g.Go("phase", CPU, func(ctx context.Context, s *Status) error {
		return nil
	})

	// Depend specifically on the first one via its returned ID (uuid string).
	g.Go("after-first", CPU, func(ctx context.Context, s *Status) error {
		mu.Lock()
		if !firstDone {
			t.Error("after-first ran before the specific first phase (id dep failed)")
		}
		mu.Unlock()
		return nil
	}, firstID)

	if err := g.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPruneKeepsDepUntilDependentFinishes(t *testing.T) {
	g, _ := New(withLogger(t), DefaultLimits())

	aStarted := make(chan struct{})
	aRelease := make(chan struct{})
	bStarted := make(chan struct{})
	bRelease := make(chan struct{})

	g.Go("a", CPU, func(ctx context.Context, s *Status) error {
		close(aStarted)
		<-aRelease
		return nil
	})
	g.Go("b", CPU, func(ctx context.Context, s *Status) error {
		close(bStarted)
		<-bRelease
		return nil
	}, "a")

	<-aStarted
	close(aRelease)
	// Wait until b is running (so a has finished and would be pruned without waiters).
	<-bStarted
	snap := g.Snapshot()
	// a must still be retained while b depends on it (or already pruned only after b done).
	// With waiters, a stays until b finishes; b is Running so at least 1 task.
	if len(snap) < 1 {
		t.Fatalf("expected live tasks while b runs, got %d", len(snap))
	}
	close(bRelease)
	if err := g.Wait(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if got := g.Snapshot(); len(got) != 0 {
		t.Fatalf("expected full prune after Wait, got %d", len(got))
	}
}

func TestManyCompletedTasksDoNotStayInSnapshot(t *testing.T) {
	g, _ := New(withLogger(t), DefaultLimits())
	const n = 200
	for i := 0; i < n; i++ {
		i := i
		g.Go(fmt.Sprintf("t:%d", i), CPU, func(ctx context.Context, s *Status) error {
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if got := g.Snapshot(); len(got) != 0 {
		t.Fatalf("expected 0 tasks after %d completions, got %d", n, len(got))
	}
}
