// Package taskgroup provides a dependency-aware task execution engine with
// resource pool limits and per-task progress reporting.
//
// It is similar to golang.org/x/sync/errgroup.Group but adds:
//   - Named tasks with Makefile-style dependency edges
//   - Three resource pools (IO, CPU, Internet) with configurable slot counts
//   - Per-task progress reporting via Status objects
//   - Nestable groups (child groups share parent pools)
//   - First-error-wins cancellation
package taskgroup

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"workspaced/pkg/logging"
)

// PoolKind identifies which resource pool a task consumes a slot from.
type PoolKind int

const (
	IO       PoolKind = iota // File system, local disk
	CPU                      // Computation
	Internet                 // Network I/O
)

func (p PoolKind) String() string {
	switch p {
	case IO:
		return "io"
	case CPU:
		return "cpu"
	case Internet:
		return "internet"
	default:
		return "unknown"
	}
}

// Limits holds the concurrency limits for each pool.
type Limits struct {
	IO       int
	CPU      int
	Internet int
}

// DefaultLimits returns sensible defaults: IO=4, CPU=NumCPU, Internet=4.
func DefaultLimits() Limits {
	cpus := max(runtime.NumCPU(), 1)
	return Limits{
		IO:       4,
		CPU:      cpus,
		Internet: 4,
	}
}

// State represents the lifecycle state of a task.
type State int

const (
	Pending State = iota
	Running
	Done
	Failed
)

func (s State) String() string {
	switch s {
	case Pending:
		return "pending"
	case Running:
		return "running"
	case Done:
		return "done"
	case Failed:
		return "failed"
	default:
		return "unknown"
	}
}

// TaskState is a point-in-time snapshot of a single task, safe to read
// concurrently from a rendering goroutine.
type TaskState struct {
	Name    string
	Pool    PoolKind
	State   State
	Message string
	Current int64
	Total   int64 // -1 means indeterminate
	Error   error
	Logs    []string
}

// Status is the handle given to a task function so it can report progress.
// All methods are safe for concurrent use.
type Status struct {
	mu      sync.Mutex
	message string
	current int64
	total   int64
	logs    []string
	// onLog is called (under lock) when a log line is added.
	onLog func(taskName, msg string)
	name  string
}

// Update sets the current status message (shown in the progress bar).
func (s *Status) Update(message string) {
	s.mu.Lock()
	s.message = message
	s.mu.Unlock()
}

// Progress sets the current/total counters. Use total=-1 for indeterminate.
func (s *Status) Progress(current, total int64) {
	s.mu.Lock()
	s.current = current
	s.total = total
	s.mu.Unlock()
}

// Log appends a message to the per-task log buffer (used by renderers for
// live output above progress bars or in plain transcripts). The message is
// also forwarded to slog via the onLog handler.
func (s *Status) Log(msg string) {
	s.mu.Lock()
	s.logs = append(s.logs, msg)
	if s.onLog != nil {
		s.onLog(s.name, msg)
	}
	s.mu.Unlock()
}

func (s *Status) snapshot() (string, int64, int64, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	logs := make([]string, len(s.logs))
	copy(logs, s.logs)
	return s.message, s.current, s.total, logs
}

// taskEntry is the internal bookkeeping for a scheduled task.
type taskEntry struct {
	name string
	pool PoolKind
	fn   func(context.Context, *Status) error
	deps []string

	// done is closed when the task finishes (success or failure).
	done chan struct{}
	// status is the progress handle for this task.
	status *Status

	mu    sync.Mutex
	state State
	err   error
}

func (t *taskEntry) setState(s State) {
	t.mu.Lock()
	t.state = s
	t.mu.Unlock()
}

func (t *taskEntry) setError(err error) {
	t.mu.Lock()
	t.state = Failed
	t.err = err
	t.mu.Unlock()
}

func (t *taskEntry) snapshot() TaskState {
	t.mu.Lock()
	state := t.state
	taskErr := t.err
	t.mu.Unlock()

	msg, cur, total, logs := t.status.snapshot()
	return TaskState{
		Name:    t.name,
		Pool:    t.pool,
		State:   state,
		Message: msg,
		Current: cur,
		Total:   total,
		Error:   taskErr,
		Logs:    logs,
	}
}

type contextKey struct{}

// pools holds the shared semaphores across nested groups.
type pools struct {
	io       chan struct{}
	cpu      chan struct{}
	internet chan struct{}
}

func newPools(limits Limits) *pools {
	return &pools{
		io:       make(chan struct{}, limits.IO),
		cpu:      make(chan struct{}, limits.CPU),
		internet: make(chan struct{}, limits.Internet),
	}
}

func (p *pools) acquire(ctx context.Context, kind PoolKind) error {
	var sem chan struct{}
	switch kind {
	case IO:
		sem = p.io
	case CPU:
		sem = p.cpu
	case Internet:
		sem = p.internet
	}
	select {
	case sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *pools) release(kind PoolKind) {
	switch kind {
	case IO:
		<-p.io
	case CPU:
		<-p.cpu
	case Internet:
		<-p.internet
	}
}

// Group coordinates dependency-aware task execution with pool limits.
type Group struct {
	mu     sync.Mutex
	tasks  []*taskEntry
	byName map[string]*taskEntry

	pools  *pools
	ctx    context.Context
	cancel context.CancelFunc

	onLog func(taskName, msg string)

	errOnce sync.Once
	err     error

	wg sync.WaitGroup
}

// New creates a root Group with the given pool limits.
// The returned context is cancelled on first task error.
func New(ctx context.Context, limits Limits) (*Group, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	g := &Group{
		byName: make(map[string]*taskEntry),
		pools:  newPools(limits),
		ctx:    ctx,
		cancel: cancel,
	}

	// Bridge Status.Log messages to slog using the logger from the
	// creation context (so task logs participate in the app's logging).
	if l := logging.GetLogger(ctx); l != nil {
		g.onLog = func(taskName, msg string) {
			l.Info(msg, "task", taskName)
		}
	}

	return g, context.WithValue(ctx, contextKey{}, g)
}

// FromContext retrieves the Group from context. Returns nil if none.
func FromContext(ctx context.Context) *Group {
	g, _ := ctx.Value(contextKey{}).(*Group)
	return g
}

// MustFromContext is like FromContext but panics if no Group is present in
// the context.
func MustFromContext(ctx context.Context) *Group {
	if g := FromContext(ctx); g != nil {
		return g
	}
	panic("taskgroup: no Group present in context; " +
		"only the top-level command may call New, everything else must receive it via context")
}

// Context returns the group's internal context. The context is cancelled
// when Wait returns or when the first task error occurs (first-error-wins).
// Renderers and other observers can select on this to know when the group
// lifecycle is over.
func (g *Group) Context() context.Context {
	return g.ctx
}

// SetLogHandler sets or chains a callback invoked for Status.Log calls.
// Used by the automatic slog bridge in New (and available for other
// observers).
func (g *Group) SetLogHandler(fn func(taskName, msg string)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.onLog == nil {
		g.onLog = fn
		return
	}
	prev := g.onLog
	g.onLog = func(name, msg string) {
		prev(name, msg)
		fn(name, msg)
	}
}

// Go schedules a named task to run in the given pool after its dependencies complete.
// Panics if the name is already registered in this group.
func (g *Group) Go(name string, pool PoolKind, fn func(ctx context.Context, s *Status) error, deps ...string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.byName[name]; exists {
		panic(fmt.Sprintf("taskgroup: duplicate task name %q", name))
	}

	t := &taskEntry{
		name: name,
		pool: pool,
		fn:   fn,
		deps: deps,
		done: make(chan struct{}),
		status: &Status{
			name:  name,
			total: -1,
			onLog: g.onLog,
		},
		state: Pending,
	}
	g.tasks = append(g.tasks, t)
	g.byName[name] = t

	g.wg.Add(1)
	go g.runTask(t)
}

func (g *Group) runTask(t *taskEntry) {
	defer g.wg.Done()
	defer close(t.done)

	// Wait for dependencies.
	for _, dep := range t.deps {
		g.mu.Lock()
		depTask, ok := g.byName[dep]
		g.mu.Unlock()
		if !ok {
			t.setError(fmt.Errorf("taskgroup: unknown dependency %q for task %q", dep, t.name))
			g.recordError(t.err)
			return
		}
		select {
		case <-depTask.done:
			// Check if dep failed.
			depTask.mu.Lock()
			depErr := depTask.err
			depTask.mu.Unlock()
			if depErr != nil {
				t.setError(fmt.Errorf("taskgroup: dependency %q failed: %w", dep, depErr))
				g.recordError(t.err)
				return
			}
		case <-g.ctx.Done():
			t.setError(g.ctx.Err())
			return
		}
	}

	// Acquire pool slot.
	if err := g.pools.acquire(g.ctx, t.pool); err != nil {
		t.setError(err)
		return
	}
	defer g.pools.release(t.pool)

	// Run the task.
	// Derive a ctx carrying a logger pre-attached with the task name so that
	// normal logging.GetLogger(ctx) calls inside the task automatically get
	// the "task" attribute.
	t.setState(Running)

	taskCtx := g.ctx
	if l := logging.GetLogger(g.ctx); l != nil {
		taskCtx = logging.ContextWithLogger(g.ctx, l.With("task", t.name))
	}

	err := t.fn(taskCtx, t.status)
	if err != nil {
		t.setError(err)
		g.recordError(err)
	} else {
		t.setState(Done)
	}
}

func (g *Group) recordError(err error) {
	g.errOnce.Do(func() {
		g.err = err
		g.cancel()
	})
}

// Wait blocks until all tasks complete and returns the first error, if any.
func (g *Group) Wait() error {
	g.wg.Wait()
	// Don't cancel on success — only on error (already done in recordError).
	// But cancel the context so renderers know we're done.
	g.cancel()
	return g.err
}

// Snapshot returns a point-in-time view of all tasks for rendering.
func (g *Group) Snapshot() []TaskState {
	g.mu.Lock()
	tasks := make([]*taskEntry, len(g.tasks))
	copy(tasks, g.tasks)
	g.mu.Unlock()

	states := make([]TaskState, len(tasks))
	for i, t := range tasks {
		states[i] = t.snapshot()
	}
	return states
}

// SubGroup creates a child Group that shares the parent's pool semaphores.
// The child's context is derived from the parent's context.
func (g *Group) SubGroup(ctx context.Context) (*Group, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	child := &Group{
		byName: make(map[string]*taskEntry),
		pools:  g.pools, // shared pools
		ctx:    ctx,
		cancel: cancel,
		onLog:  g.onLog,
	}
	return child, context.WithValue(ctx, contextKey{}, child)
}
