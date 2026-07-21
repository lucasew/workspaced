// Package taskgroup provides a dependency-aware task execution engine with
// resource pool limits and per-task progress reporting.
//
// It is similar to golang.org/x/sync/errgroup.Group but adds:
//   - Named tasks with Makefile-style dependency edges
//   - Three resource pools (IO, CPU, Internet) with configurable slot counts
//   - Per-task progress reporting via Status objects
//   - Nestable groups (child groups share parent pools)
//   - First-error-wins cancellation
//   - Pruning of completed tasks (no waiter deps left) so Snapshot/TUI stay O(live work)
//
// # Progress hierarchy
//
// One Session owns one root Group and the optional TUI. Prefer a single owner
// for each unit of visible progress:
//
//   - Session root: command lifetime (Enter / Close); do not add bars here.
//   - Command Control task: optional anchor with Status.Unit when the command
//     is one logical step and nested work has no aggregate bar of its own.
//   - Map[T,U].Run: fan-out that collects results under one aggregate bar.
//     Serial: true keeps the bar but runs one item at a time (e.g. formatters).
//   - Each[T].Run: same fan-out when only errors matter (no struct{} results).
//     Children should not also call Unit (avoids N stuck 0/1 bars).
//   - Isolate / GoIsolated: error boundary SubGroup so a failure does not
//     cancel parent siblings. Isolate alone adds no bar; GoIsolated adds one
//     named task. Do not GoIsolated around HTTP-only work when
//     httpclient.WithProgress already promotes the request to a fetch task.
//   - Raw SubGroup: escape hatch; prefer Isolate, GoIsolated, Map, or Each.
//
// Stacking Control+Unit around Map/Each.Run, or SubGroup+Go+Unit around a
// fetch task, duplicates bars without adding information.
package taskgroup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sort"
	"sync"

	"github.com/google/uuid"

	"github.com/lucasew/workspaced/pkg/logging"
)

var (
	ErrUnknownDependency = errors.New("unknown dependency")
	ErrDependencyFailed  = errors.New("dependency failed")
	// ErrNilFn is returned by Map.Run / Each.Run when Fn is unset.
	ErrNilFn = errors.New("Fn is nil")
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
	ID      string // UUIDv7 unique key for this task instance
	Name    string // Description (human label passed to Go); not unique
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
// The TUI only renders bars for running tasks with total > 0.
func (s *Status) Progress(current, total int64) {
	s.mu.Lock()
	s.current = current
	s.total = total
	s.mu.Unlock()
}

// Unit marks this task as a single-step item (0/1) so the TUI shows a bar.
// Returns a done func that sets 1/1; call via defer s.Unit()().
//
// Skip Unit when a nested task already owns real progress (Map aggregate,
// httpclient byte progress, rsync stats). Stacking Unit on those parents
// creates duplicate hierarchy bars that sit at 0% beside the real work.
func (s *Status) Unit() (done func()) {
	s.Progress(0, 1)
	return func() { s.Progress(1, 1) }
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
	id   string // UUIDv7
	desc string // description (display label, may not be unique)
	pool PoolKind
	fn   func(context.Context, *Status) error
	deps []string

	// resolvedDeps are the concrete dep tasks pinned at Go() time (by id or
	// latest matching description). Used to release waiter pins on finish.
	resolvedDeps []*taskEntry

	// waiters is the number of not-yet-finished tasks that listed this task
	// as a dependency. Protected by Group.mu. Completed tasks with waiters==0
	// are pruned from the group so Snapshot/TUI do not retain them forever.
	waiters int

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
		ID:      t.id,
		Name:    t.desc,
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
	mu   sync.Mutex
	Live TaskCollection // Live tasks; protected by mu (see task_collection.go)

	pools  *pools
	ctx    context.Context
	cancel context.CancelFunc

	errOnce sync.Once
	err     error

	wg sync.WaitGroup

	onLog func(taskName, msg string)

	// usingBubbleTea is set while a session progress UI is active
	// for this group. It tells per-task logRecorders to skip normal slog delegate
	// (the renderer owns visible emission via prog.Printf) to avoid duplicate lines.
	usingBubbleTea bool

	// children are SubGroups. Used so that SetLogHandler / setUsingBubbleTea
	// can propagate the interceptor settings (onLog + bubbletea skip flag)
	// to subgroups and their tasks. This ensures the logger interceptor
	// (logRecorder + onLog callback for prog.Printf + usingBubbleTea skip)
	// is taken for work created via Map / late SubGroups.
	children []*Group

	// onSchedule is invoked once per Go() (including on SubGroups). Used by
	// Session to lazily start the progress UI on the first scheduled task.
	onSchedule func()

	// session is the owning Session for this tree (set by Enter; copied to SubGroups).
	// Used so task contexts can apply Session.Overlay even though g.ctx itself
	// does not carry sessionKey.
	session *Session
}

// New creates a root Group with the given pool limits.
// The returned context is cancelled on first task error.
func New(ctx context.Context, limits Limits) (*Group, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	g := &Group{
		Live:   NewTaskCollection(),
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
// session UI) in addition to the per-task Logs slices in Snapshot().
//
// It updates the group, direct tasks, *and* any SubGroups (recursively) plus
// their tasks. This ensures the logger interceptor (logRecorder.append that
// feeds onLog + the usingBubbleTea skip in Handle) is taken for work scheduled
// via Map / SubGroup, even if those subgroups were created before or around
// the time the session UI wires the handler.
func (g *Group) SetLogHandler(fn func(taskName, msg string)) {
	g.mu.Lock()
	g.onLog = fn
	g.Live.ForEach(func(t *taskEntry) {
		if t.status != nil {
			t.status.mu.Lock()
			t.status.onLog = fn
			t.status.mu.Unlock()
		}
	})
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
	g.Live.ForEach(func(t *taskEntry) {
		if t.status != nil {
			t.status.mu.Lock()
			t.status.onLog = fn
			t.status.mu.Unlock()
		}
	})
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

// Go schedules a task to run in the given pool after its dependencies complete.
// The desc is a human-readable description (display label) and is not required
// to be unique; the same description may be used for multiple tasks.
// Internally a UUIDv7 is used as the unique key.
//
// Go returns the task's UUIDv7 string key. This key can be passed in a later
// Go call's deps list for an exact dependency (preferred when descriptions
// may repeat). For convenience, a dep string that is not a known key is
// interpreted as a description and resolves to the most recently scheduled
// task (in this group) with a matching description.
func (g *Group) Go(desc string, pool PoolKind, fn func(ctx context.Context, s *Status) error, deps ...string) string {
	if g.onSchedule != nil {
		g.onSchedule()
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	id := uuid.Must(uuid.NewV7()).String()

	t := &taskEntry{
		id:   id,
		desc: desc,
		pool: pool,
		fn:   fn,
		deps: deps,
		done: make(chan struct{}),
		status: &Status{
			name:  desc,
			total: -1,
			onLog: g.onLog,
		},
		state: Pending,
	}

	// Pin dependencies while we still hold the group lock so a fast-finishing
	// dep is not pruned before this task can wait on it.
	g.Live.PinDeps(t, deps)
	g.Live.Add(t)

	g.wg.Add(1)
	go g.runTask(t)
	return id
}

// finishTask closes the done channel, releases dependency waiter pins, and
// prunes this task (and any deps that are now unreferenced) from the group.
func (g *Group) finishTask(t *taskEntry) {
	close(t.done)

	g.mu.Lock()
	defer g.mu.Unlock()
	g.Live.ReleaseAndPrune(t)
}

// logRecorder is a slog.Handler wrapper used for task execution.
// It appends a formatted version (using logging.FormatPrepend so that
// progress-bar / bubbletea systems get the same plain-or-colored output
// as the main PlainHandler) to the task's log buffer and calls any onLog
// callback. When bubbletea is active it skips the normal delegate to avoid
// duplicate lines.
type logRecorder struct {
	slog.Handler
	append func(string)
	group  *Group // for runtime isUsingBubbleTea check (supports opt-in after schedule)
	attrs  []slog.Attr
}

func (r *logRecorder) Handle(ctx context.Context, rec slog.Record) error {
	if r.append != nil {
		r.append(logging.FormatPrepend(rec, r.attrs...))
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
	defer g.finishTask(t)

	// Dependencies were pinned at Go() when possible. Unknown names leave
	// resolvedDeps shorter than deps and fail here.
	if len(t.deps) > 0 && len(t.resolvedDeps) < len(t.deps) {
		missing := t.deps[len(t.resolvedDeps)]
		for _, dep := range t.deps {
			found := false
			for _, d := range t.resolvedDeps {
				if d.id == dep || d.desc == dep {
					found = true
					break
				}
			}
			if !found {
				missing = dep
				break
			}
		}
		t.setError(fmt.Errorf("taskgroup: %w %q for task %q", ErrUnknownDependency, missing, t.desc))
		g.recordError(t.err)
		return
	}
	for i, depTask := range t.resolvedDeps {
		depLabel := depTask.desc
		if i < len(t.deps) {
			depLabel = t.deps[i]
		}
		select {
		case <-depTask.done:
			depTask.mu.Lock()
			depErr := depTask.err
			depTask.mu.Unlock()
			if depErr != nil {
				t.setError(fmt.Errorf("taskgroup: %w: %q: %w", ErrDependencyFailed, depLabel, depErr))
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
	// Session.Overlay values (e.g. plan's dry-run set after Enter) must be
	// visible to materializers that only see this task ctx.
	if sess := g.session; sess != nil {
		if o := sess.overlayContext(); o != nil {
			taskCtx = valueOverlay{Context: taskCtx, overlay: o}
		}
	}
	if base := logging.GetLogger(g.ctx); base != nil {
		tagged := base.With("task", t.desc)
		rec := &logRecorder{
			Handler: tagged.Handler(),
			group:   g,
			attrs:   []slog.Attr{slog.String("task", t.desc)},
			append: func(msg string) {
				t.status.mu.Lock()
				t.status.logs = append(t.status.logs, msg)
				onLog := t.status.onLog
				t.status.mu.Unlock()
				if onLog != nil {
					onLog(t.desc, msg)
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
						onLog(t.desc, msg)
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
	// Only cancel on error (to abort other waiting tasks/pools).
	// On success we no longer cancel here; renderers now drive their own
	// lifetime via explicit waiter + prog.Quit() (avoids returning "context canceled"
	// from tea programs that use WithContext(g.ctx)).
	if g.err != nil {
		g.cancel()
	}
	return g.err
}

// Snapshot returns a point-in-time view of all live tasks for rendering.
// Order is unspecified (map iteration); the bubbletea UI keeps its own
// first-seen order for progress bars.
func (g *Group) Snapshot() []TaskState {
	g.mu.Lock()
	tasks := g.Live.Entries()
	g.mu.Unlock()

	states := make([]TaskState, len(tasks))
	for i, t := range tasks {
		states[i] = t.snapshot()
	}
	return states
}

// snapshotRecursive returns tasks from this group and all descendant SubGroups
// (recursively). This allows the bubbletea progress renderer (and similar
// observers) to see work that was intentionally scheduled via SubGroup
// (e.g. the "install:..." and inner "fetch:..." tasks created during
// tool EnsureInstalled / manager.Install flows) so that detailed download
// and install progress bars are still visible in the UI.
func (g *Group) snapshotRecursive() []TaskState {
	var out []TaskState

	var walk func(*Group)
	walk = func(gg *Group) {
		snap := gg.Snapshot()
		out = append(out, snap...)

		gg.mu.Lock()
		kids := make([]*Group, len(gg.children))
		copy(kids, gg.children)
		gg.mu.Unlock()

		for _, k := range kids {
			walk(k)
		}
	}

	walk(g)
	return out
}

// SnapshotSorted returns the tasks from Snapshot, sorted stably for
// predictable UI ordering: first by PoolKind, then by Name (description),
// then by ID for determinism when descriptions repeat.
func (g *Group) SnapshotSorted() []TaskState {
	snap := g.Snapshot()
	sort.SliceStable(snap, func(i, j int) bool {
		if snap[i].Pool != snap[j].Pool {
			return snap[i].Pool < snap[j].Pool
		}
		if snap[i].Name != snap[j].Name {
			return snap[i].Name < snap[j].Name
		}
		return snap[i].ID < snap[j].ID
	})
	return snap
}

// Isolate runs fn on a SubGroup derived from ctx's Group. Failures cancel only
// that subgroup; parent siblings keep running. Tasks fn schedules via
// MustFromContext are waited on before return. When no Group is on ctx, fn
// runs on ctx unchanged.
//
// Isolate adds an error boundary without an extra progress bar. Prefer Map or
// Each for fan-out with aggregate progress, or GoIsolated for one named task.
func Isolate(ctx context.Context, fn func(context.Context) error) error {
	if fn == nil {
		return nil
	}
	parent := FromContext(ctx)
	if parent == nil {
		return fn(ctx)
	}
	child, childCtx := parent.SubGroup(ctx)
	fnErr := fn(childCtx)
	if werr := child.Wait(); werr != nil && fnErr == nil {
		fnErr = werr
	}
	return fnErr
}

// GoIsolated schedules one named task on an isolated SubGroup, waits for it,
// and returns its error without cancelling sibling work on the parent group.
// When no Group is on ctx, fn runs synchronously with a throwaway Status.
//
// Do not wrap HTTP-only work this way when httpclient.WithProgress already
// promotes the request to an Internet task — that yields two bars (unit shell
// + fetch). Use Isolate(ctx, perform) so only the fetch task appears.
func GoIsolated(ctx context.Context, name string, pool PoolKind, fn func(context.Context, *Status) error) error {
	if fn == nil {
		return nil
	}
	if FromContext(ctx) == nil {
		return fn(ctx, &Status{name: name, total: -1})
	}
	return Isolate(ctx, func(ctx context.Context) error {
		g := MustFromContext(ctx)
		var taskErr error
		g.Go(name, pool, func(ctx context.Context, s *Status) error {
			taskErr = fn(ctx, s)
			return taskErr
		})
		if werr := g.Wait(); werr != nil && taskErr == nil {
			return werr
		}
		return taskErr
	})
}

// SubGroup creates a child Group that shares the parent's pool semaphores.
// The child's context is derived from the parent's context.
//
// Prefer Isolate, GoIsolated, Map, or Each unless you need the raw child Group
// handle (e.g. dependency edges across manually scheduled tasks).
func (g *Group) SubGroup(ctx context.Context) (*Group, context.Context) {
	cctx, cancel := context.WithCancel(ctx)

	// Snapshot the current logger interceptor settings (onLog + usingBubbleTea)
	// under the lock. This lets subgroups created after the session UI has
	// wired the handler still get the interceptor (the append callback for
	// prog.Printf + the skip in logRecorder.Handle).
	g.mu.Lock()
	onLogCopy := g.onLog
	usingCopy := g.usingBubbleTea
	g.mu.Unlock()

	child := &Group{
		Live:           NewTaskCollection(),
		pools:          g.pools, // shared pools
		ctx:            cctx,
		cancel:         cancel,
		onLog:          onLogCopy,
		usingBubbleTea: usingCopy,
		onSchedule:     g.onSchedule, // lazy session UI on first Go anywhere in the tree
		session:        g.session,
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
	// Preserve Session from the parent ctx when present so MustSessionFrom works.
	childCtx := context.WithValue(cctx, contextKey{}, child)
	if sess := SessionFrom(ctx); sess != nil {
		childCtx = context.WithValue(childCtx, sessionKey{}, sess)
	}
	return child, childCtx
}

// valueOverlay prefers overlay.Value for lookups while keeping base deadline/cancel.
type valueOverlay struct {
	context.Context
	overlay context.Context
}

func (v valueOverlay) Value(key any) any {
	if v.overlay != nil {
		if val := v.overlay.Value(key); val != nil {
			return val
		}
	}
	return v.Context.Value(key)
}
