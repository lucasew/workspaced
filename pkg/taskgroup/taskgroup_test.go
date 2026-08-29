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

	"github.com/lucasew/workspaced/pkg/logging"
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

	for i := range 10 {
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
	if FromContext(t.Context()) != nil {
		t.Fatal("FromContext on empty context should return nil")
	}
}

func TestMap_BasicAndOrder(t *testing.T) {
	g, ctx := New(withLogger(t), DefaultLimits())
	_ = g

	input := []int{1, 2, 3, 4, 5}

	results, err := Map[int, int]{
		Items:    input,
		PoolKind: CPU,
		TaskName: func(i int, _ int) string { return fmt.Sprintf("map:%d", i) },
		Fn: func(ctx context.Context, s *Status, v int) (int, error) {
			s.Update(fmt.Sprintf("processing %d", v))
			time.Sleep(1 * time.Millisecond)
			return v * 10, nil
		},
	}.Run(ctx)
	if err != nil {
		t.Fatalf("Map.Run returned error: %v", err)
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
}

func TestMap_SerialRunsOneAtATime(t *testing.T) {
	g, ctx := New(withLogger(t), DefaultLimits())
	_ = g

	var (
		mu           sync.Mutex
		inFlight     int
		maxInFlight  int
		startedOrder []int
	)

	input := []int{1, 2, 3, 4}
	results, err := Map[int, int]{
		Items:    input,
		PoolKind: CPU,
		Serial:   true,
		Fn: func(ctx context.Context, s *Status, v int) (int, error) {
			mu.Lock()
			inFlight++
			if inFlight > maxInFlight {
				maxInFlight = inFlight
			}
			startedOrder = append(startedOrder, v)
			mu.Unlock()

			time.Sleep(20 * time.Millisecond)

			mu.Lock()
			inFlight--
			mu.Unlock()
			return v, nil
		},
	}.Run(ctx)
	if err != nil {
		t.Fatalf("Map.Run: %v", err)
	}
	if maxInFlight != 1 {
		t.Fatalf("max concurrent %d, want 1", maxInFlight)
	}
	if len(results) != len(input) {
		t.Fatalf("results len %d, want %d", len(results), len(input))
	}
	for i, v := range startedOrder {
		if v != input[i] {
			t.Fatalf("started order %v, want %v", startedOrder, input)
		}
	}
}

func TestMap_Empty(t *testing.T) {
	g, ctx := New(withLogger(t), DefaultLimits())
	_ = g

	results, err := Map[string, string]{
		Items:    nil,
		PoolKind: IO,
		Fn:       func(ctx context.Context, s *Status, v string) (string, error) { return v + "!", nil },
	}.Run(ctx)
	if err != nil {
		t.Fatalf("empty Map.Run error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty result slice, got len=%d", len(results))
	}
}

func TestMap_NilFn(t *testing.T) {
	_, ctx := New(withLogger(t), DefaultLimits())
	_, err := Map[int, int]{Items: []int{1}}.Run(ctx)
	if !errors.Is(err, ErrNilFn) {
		t.Fatalf("got %v, want ErrNilFn", err)
	}
}

func TestEach_RunsWithoutResults(t *testing.T) {
	g, ctx := New(withLogger(t), DefaultLimits())
	_ = g

	var saw atomic.Int64
	err := Each[int]{
		Name:     "each",
		Items:    []int{1, 2, 3},
		PoolKind: CPU,
		Fn: func(ctx context.Context, s *Status, v int) error {
			saw.Add(int64(v))
			return nil
		},
	}.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if saw.Load() != 6 {
		t.Fatalf("saw sum %d, want 6", saw.Load())
	}
}

func TestEach_NilFn(t *testing.T) {
	_, ctx := New(withLogger(t), DefaultLimits())
	if err := (Each[int]{Items: []int{1}}).Run(ctx); !errors.Is(err, ErrNilFn) {
		t.Fatalf("got %v, want ErrNilFn", err)
	}
}

func TestMap_ErrorPropagates(t *testing.T) {
	g, ctx := New(withLogger(t), DefaultLimits())
	_ = g

	_, err := Map[int, int]{
		Items:    []int{10, 20, 30},
		PoolKind: IO,
		Fn: func(ctx context.Context, s *Status, v int) (int, error) {
			if v == 20 {
				return 0, errors.New("boom on 20")
			}
			return v, nil
		},
	}.Run(ctx)
	if err == nil {
		t.Fatal("expected error from Map.Run, got nil")
	}
}

func TestMap_UsesProvidedTaskNames(t *testing.T) {
	g, ctx := New(withLogger(t), DefaultLimits())
	_ = g

	items := []string{"a.txt", "b.txt"}

	results, err := Map[string, string]{
		Items:    items,
		PoolKind: IO,
		TaskName: func(_ int, name string) string { return "read:" + name },
		Fn: func(ctx context.Context, s *Status, name string) (string, error) {
			return "content-of-" + name, nil
		},
	}.Run(ctx)
	if err != nil {
		t.Fatalf("named Map.Run failed: %v", err)
	}
	if len(results) != len(items) {
		t.Fatalf("expected %d results, got %d", len(items), len(results))
	}
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
	for i := range n {
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

func TestStatusUnit(t *testing.T) {
	var s Status
	done := s.Unit()
	_, cur, total, _ := s.snapshot()
	if cur != 0 || total != 1 {
		t.Fatalf("Unit start: cur=%d total=%d", cur, total)
	}
	done()
	_, cur, total, _ = s.snapshot()
	if cur != 1 || total != 1 {
		t.Fatalf("Unit done: cur=%d total=%d", cur, total)
	}
}

func TestIsolateDoesNotCancelParentSiblings(t *testing.T) {
	g, ctx := New(withLogger(t), DefaultLimits())
	release := make(chan struct{})
	var siblingRan atomic.Bool

	g.Go("sibling", CPU, func(ctx context.Context, s *Status) error {
		<-release
		siblingRan.Store(true)
		return nil
	})

	err := Isolate(ctx, func(ctx context.Context) error {
		child := MustFromContext(ctx)
		child.Go("boom", CPU, func(ctx context.Context, s *Status) error {
			return errors.New("isolated failure")
		})
		return child.Wait()
	})
	if err == nil {
		t.Fatal("expected isolated failure")
	}
	close(release)
	if werr := g.Wait(); werr != nil {
		t.Fatalf("parent group should succeed: %v", werr)
	}
	if !siblingRan.Load() {
		t.Fatal("parent sibling should have run")
	}
}

func TestGoIsolatedWithoutGroupRunsSync(t *testing.T) {
	var ran bool
	err := GoIsolated(withLogger(t), "x", CPU, func(ctx context.Context, s *Status) error {
		ran = true
		if s == nil {
			t.Fatal("status is nil")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("fn did not run")
	}
}

func TestMapNamedOrchestrator(t *testing.T) {
	g, ctx := New(withLogger(t), DefaultLimits())
	started := make(chan struct{})
	release := make(chan struct{})

	go func() {
		_, err := Map[int, int]{
			Name:     "plan",
			Items:    []int{1},
			PoolKind: CPU,
			Fn: func(ctx context.Context, s *Status, v int) (int, error) {
				close(started)
				<-release
				return v, nil
			},
		}.Run(ctx)
		if err != nil {
			t.Errorf("Map.Run: %v", err)
		}
	}()
	<-started
	found := false
	for _, ts := range g.snapshotRecursive() {
		if ts.Name == "plan" && ts.State == Running && ts.Total == 1 {
			found = true
			break
		}
	}
	close(release)
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected running orchestrator named plan with total=1")
	}
}
