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
	"log/slog"
	"runtime"
	"sync"

	"workspaced/pkg/logging"
)

// PoolKind identifies which resource pool a task consumes a slot from.
type PoolKind int

const (
	Control  PoolKind = iota // Unlimited, used to create other tasks
	IO                       // File system, local disk
	CPU                      // Computation
	Internet                 // Network I/O
)

func (p PoolKind) String() string {
	switch p {
	case Control:
		return "control"
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
	onLog   func(taskName, msg string)
	name    string
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
	case Control:
		// Control tasks are for orchestration. They do not consume a limited
		// resource pool and must never block on acquire.
		return nil
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
	case Control:
		// No slot was acquired.
		return
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

	errOnce sync.Once
	err     error

	wg sync.WaitGroup

	onLog func(taskName, msg string)

	// usingBubbleTea is set while a bubbletea renderer (RunBubbleTea) is active
	// for this group. It tells per-task logRecorders to skip normal slog delegate
	// (the renderer owns visible emission via prog.Printf) to avoid duplicate lines.
	usingBubbleTea bool

	// children are SubGroups. Used so that SetLogHandler / setUsingBubbleTea
	// can propagate the interceptor settings (onLog + bubbletea skip flag)
	// to subgroups and their tasks. This ensures the logger interceptor
	// (logRecorder + onLog callback for prog.Printf + usingBubbleTea skip)
	// is taken for work created via Map / late SubGroups.
	children []*Group
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

	return g, context.WithValue(ctx, contextKey{}, g)
}

// FromContext retrieves the Group from context. Returns nil if none.
func FromContext(ctx context.Context) *Group {
	g, _ := ctx.Value(contextKey{}).(*Group)
	return g
}

// SetLogHandler sets a callback invoked whenever a log is recorded for a task
// (via the context logger inside the task func, which feeds the recorder).
// The callback is useful for real-time observers (e.g. TUI renderers like
// RunBubbleTea) in addition to the per-task Logs slices in Snapshot().
//
// It updates the group, direct tasks, *and* any SubGroups (recursively) plus
// their tasks. This ensures the logger interceptor (logRecorder.append that
// feeds onLog + the usingBubbleTea skip in Handle) is taken for work scheduled
// via Map / SubGroup, even if those subgroups were created before or around
// the time RunBubbleTea wires the handler (the common opt-in pattern).
func (g *Group) SetLogHandler(fn func(taskName, msg string)) {
	g.mu.Lock()
	g.onLog = fn
	for _, t := range g.tasks {
		if t.status != nil {
			t.status.mu.Lock()
			t.status.onLog = fn
			t.status.mu.Unlock()
		}
	}
	for _, ch := range g.children {
		ch.propagateLogHandler(fn)
	}
	g.mu.Unlock()
}

// propagateLogHandler is called on children when an ancestor sets a new
// log handler (via SetLogHandler on root or intermediate group). It
// ensures the logger interceptor (onLog callback to prog.Printf + refresh,
// and the usingBubbleTea skip decision) reaches tasks created inside
// SubGroups / via Map even if those subgroups were created before or
// around the time the renderer was activated.
func (g *Group) propagateLogHandler(fn func(taskName, msg string)) {
	g.mu.Lock()
	g.onLog = fn
	for _, t := range g.tasks {
		if t.status != nil {
			t.status.mu.Lock()
			t.status.onLog = fn
			t.status.mu.Unlock()
		}
	}
	for _, ch := range g.children {
		ch.propagateLogHandler(fn)
	}
	g.mu.Unlock()
}

// setUsingBubbleTea marks that a bubbletea renderer is driving output for this
// group (used by logRecorder to decide whether to skip normal delegate).
// It propagates to any SubGroups (children) so that logRecorders created
// for tasks inside Map / nested SubGroups will also take the interceptor
// path (skip normal output + feed onLog for the TUI).
func (g *Group) setUsingBubbleTea(v bool) {
	g.mu.Lock()
	g.usingBubbleTea = v
	for _, ch := range g.children {
		ch.setUsingBubbleTeaFromAncestor(v)
	}
	g.mu.Unlock()
}

func (g *Group) setUsingBubbleTeaFromAncestor(v bool) {
	g.mu.Lock()
	g.usingBubbleTea = v
	for _, ch := range g.children {
		ch.setUsingBubbleTeaFromAncestor(v)
	}
	g.mu.Unlock()
}

// isUsingBubbleTea reports whether a bubbletea renderer currently owns emission.
func (g *Group) isUsingBubbleTea() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.usingBubbleTea
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

// logRecorder is a slog.Handler wrapper used for task execution.
// It appends a formatted version (message + attrs) to the task's log buffer
// (for Snapshot + any attached renderer) and delegates to the real handler
// for normal slog output. When a bubbletea renderer is active on the group
// (see RunBubbleTea), it skips the delegate and lets the renderer's
// prog.Printf path own the visible emission (prevents duplicate lines).
type logRecorder struct {
	slog.Handler
	append func(string)
	group  *Group // for runtime isUsingBubbleTea check (supports opt-in after schedule)
	attrs  []slog.Attr
}

func (r *logRecorder) Handle(ctx context.Context, rec slog.Record) error {
	if r.append != nil {
		r.append(logging.FormatPlainPrepend(rec, r.attrs...))
	}
	// Skip normal delegate while a bubbletea renderer owns visible output
	// (it uses prog.Printf on the tea writer so logs scroll naturally and
	// bars are redrawn below). This prevents duplicate prints for TUI demos.
	// When no renderer is active (normal commands), we always delegate so
	// context-logger calls inside tasks produce real slog output.
	if r.group != nil && r.group.isUsingBubbleTea() {
		return nil
	}
	if r.Handler != nil {
		return r.Handler.Handle(ctx, rec)
	}
	return nil
}

func (r *logRecorder) WithAttrs(attrs []slog.Attr) slog.Handler {
	nextAttrs := make([]slog.Attr, 0, len(r.attrs)+len(attrs))
	nextAttrs = append(nextAttrs, r.attrs...)
	nextAttrs = append(nextAttrs, attrs...)
	return &logRecorder{
		Handler: r.Handler.WithAttrs(attrs),
		append:  r.append,
		group:   r.group,
		attrs:   nextAttrs,
	}
}

func (r *logRecorder) WithGroup(name string) slog.Handler {
	return &logRecorder{
		Handler: r.Handler.WithGroup(name),
		append:  r.append,
		group:   r.group,
		attrs:   r.attrs,
	}
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
	// We set up the logger for this task so that calls to
	// logging.GetLogger(ctx) (or slog via the stored logger) automatically:
	//   - get a "task" attribute
	//   - have their messages recorded into the task's log buffer (for the
	//     progress renderer to display above bars / in transcripts)
	// This replaces the old Status.Log / g.Log path.
	t.setState(Running)

	// Thread the current group into the task context so that code running inside
	// a task (e.g. taskgroup.Map, or anything that does MustFromContext) can
	// retrieve the group this work is associated with and create SubGroups etc.
	// Previously the per-task ctx only derived from g.ctx (the internal cancel ctx)
	// which did not carry the contextKey, only the command-level context did.
	taskCtx := context.WithValue(g.ctx, contextKey{}, g)
	if base := logging.GetLogger(g.ctx); base != nil {
		tagged := base.With("task", t.name)
		rec := &logRecorder{
			Handler: tagged.Handler(),
			group:   g,
			attrs:   []slog.Attr{slog.String("task", t.name)},
			append: func(msg string) {
				t.status.mu.Lock()
				t.status.logs = append(t.status.logs, msg)
				onLog := t.status.onLog
				t.status.mu.Unlock()
				if onLog != nil {
					onLog(t.name, msg)
					return
				}
				// Belt-and-suspenders fallback: consult the group that owns
				// this task (closed over as `g`). This catches cases where a
				// status was created before a SetLogHandler (or its propagation
				// to SubGroups via children) ran, e.g. subgroups created inside
				// Map after the root renderer wired the handler. This makes
				// sure the logger interceptor (onLog callback) is taken.
				if g != nil {
					g.mu.Lock()
					onLog = g.onLog
					g.mu.Unlock()
					if onLog != nil {
						onLog(t.name, msg)
					}
				}
			},
		}
		taskLogger := slog.New(rec)
		taskCtx = logging.ContextWithLogger(taskCtx, taskLogger)
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
	cctx, cancel := context.WithCancel(ctx)

	// Snapshot the current logger interceptor settings (onLog + usingBubbleTea)
	// under the lock. This lets subgroups created after RunBubbleTea has
	// wired the handler still get the interceptor (the append callback for
	// prog.Printf + the skip in logRecorder.Handle).
	g.mu.Lock()
	onLogCopy := g.onLog
	usingCopy := g.usingBubbleTea
	g.mu.Unlock()

	child := &Group{
		byName:         make(map[string]*taskEntry),
		pools:          g.pools, // shared pools
		ctx:            cctx,
		cancel:         cancel,
		onLog:          onLogCopy,
		usingBubbleTea: usingCopy,
	}

	// Register the child so SetLogHandler / setUsingBubbleTea on ancestors
	// will propagate the logger interceptor (onLog for the TUI + the
	// usingBubbleTea skip flag in logRecorder) to it and its tasks.
	g.mu.Lock()
	g.children = append(g.children, child)
	g.mu.Unlock()

	// Attach the child group to the context so MustFromContext works on
	// contexts derived from the child's .ctx (in addition to the explicit
	// WithValue returned here, and the forcing we do in runTask).
	childCtx := context.WithValue(cctx, contextKey{}, child)
	return child, childCtx
}

// Map executes the handler for every item in the slice, using the task Group
// from ctx (via MustFromContext). Items run concurrently subject to the chosen
// pool's concurrency limit. Results are returned in the same order as items.
//
// This is the core "parallel map" primitive over the taskgroup system.
// The length of the input list is the natural progress total ("progressbar hint").
//
//   - pool: which resource pool to consume slots from (CPU / IO / Internet).
//   - taskName: produces a stable name for the per-item task (shown in logs,
//     Snapshot(), and the bubbletea renderer). Return "" for a default.
//   - handler: receives its own per-item *Status. Use s.Update(...) for messages
//     and s.Progress(cur, tot) for item-specific progress (the Map wrapper seeds
//     Progress(0, 1) as a unit-of-work hint).
//
// Progress hint usage (typical pattern when Map is called from inside another task):
//
//	s.Progress(0, int64(len(items))) // outer bar knows the total
//	outs, err := Map(ctx, IO, items, func(i int, it T) string { return "work:" + it.Name }, handler)
//	// on return the outer status can be advanced to len(outs) if desired
//
// The per-item tasks will each have a small progress total so the TUI can draw
// bars for the currently in-flight work items (bounded by the pool size).
func Map[T any, U any](
	ctx context.Context,
	pool PoolKind,
	items []T,
	taskName func(int, T) string,
	handler func(ctx context.Context, s *Status, item T) (U, error),
) ([]U, error) {
	if len(items) == 0 {
		return []U{}, nil
	}

	parent := MustFromContext(ctx)
	g, _ := parent.SubGroup(ctx)

	results := make([]U, len(items))

	for i := range items {
		i := i
		item := items[i]

		name := ""
		if taskName != nil {
			name = taskName(i, item)
		}
		if name == "" {
			name = fmt.Sprintf("map:%d", i)
		}

		g.Go(name, pool, func(ctx context.Context, s *Status) error {
			// Seed a unit total so progress bars have something to show for this item.
			// Individual handlers can call s.Progress with more specific (bytes, total)
			// numbers if the work item itself is incremental.
			s.Progress(0, 1)

			u, err := handler(ctx, s, item)
			if err != nil {
				return err
			}
			results[i] = u
			s.Progress(1, 1)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}
