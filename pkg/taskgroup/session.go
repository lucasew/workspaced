package taskgroup

import (
	"context"
	"os"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasew/workspaced/pkg/logging"
)

type sessionKey struct{}

// Session owns the CLI task runtime for one command invocation: the root
// Group, optional progress UI, and global output redirections. Enter in
// PersistentPreRun; Close in PersistentPostRun (idempotent; also safe to call
// early if a command must finish the session before RunE returns).
//
// Progress UI starts lazily on the first Group.Go (including via SubGroup/Map)
// when the terminal is interactive — commands that only use AfterWait or print
// stdout (e.g. history search for Ctrl+R) never take over the tty.
//
// Commands should only schedule work with Group.Go (via MustFromContext) and
// register AfterWait for anything that needs real stdio or must run after all
// tasks (plan tables, stdout paths, child process exec).
type Session struct {
	group *Group

	// wantUI is set at Enter when the terminal is interactive; startUI runs
	// at most once on the first scheduled task.
	wantUI bool
	uiOnce sync.Once
	prog   *tea.Program
	uiDone chan struct{}
	uiErr  error
	out    *outputEnv

	mu    sync.Mutex
	after []func() error

	// overlay holds context values merged into every task ctx (Value only).
	// Used when a subcommand sets flags after Enter (e.g. plan forces dry-run).
	overlay context.Context

	closeOnce sync.Once
	err       error
}

// Enter creates a root Group. UI is not started yet (see ensureUI).
// The returned context carries both the Group (MustFromContext) and Session.
func Enter(ctx context.Context, limits Limits) (*Session, context.Context) {
	g, ctx := New(ctx, limits)
	s := &Session{
		group:  g,
		wantUI: isInteractiveTerminal(),
	}
	// Notify on any Go in this tree (root and SubGroups share the same session
	// lookup via SessionFrom on the group context).
	g.onSchedule = s.ensureUI
	g.session = s
	ctx = context.WithValue(ctx, sessionKey{}, s)
	return s, ctx
}

// SessionFrom returns the Session attached to ctx, or nil.
func SessionFrom(ctx context.Context) *Session {
	s, _ := ctx.Value(sessionKey{}).(*Session)
	return s
}

// MustSessionFrom panics if no Session is on ctx.
func MustSessionFrom(ctx context.Context) *Session {
	s := SessionFrom(ctx)
	if s == nil {
		panic("taskgroup: no Session in context (Enter was not called)")
	}
	return s
}

// Group returns the root task group for this session.
func (s *Session) Group() *Group {
	if s == nil {
		return nil
	}
	return s.group
}

// Overlay records ctx values to merge into every subsequent task context.
// Call after Enter when a subcommand needs flags visible inside tasks
// (e.g. home/codebase plan forces dry-run after the root session started).
// Only context.Value lookups are overlaid; cancellation still follows the group.
func (s *Session) Overlay(ctx context.Context) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.overlay = ctx
	s.mu.Unlock()
}

func (s *Session) overlayContext() context.Context {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.overlay
}

// AfterWait registers fn to run after all tasks complete and the UI/output
// environment have been torn down (real stderr is restored). Safe to call
// from RunE while scheduling work. Hooks run in registration order.
// The first non-nil error from a hook becomes the Close result when the task
// group itself succeeded (e.g. child exit status, report write failures).
func (s *Session) AfterWait(fn func() error) {
	if s == nil || fn == nil {
		return
	}
	s.mu.Lock()
	s.after = append(s.after, fn)
	s.mu.Unlock()
}

// ensureUI starts the progress UI on first scheduled task (lazy).
func (s *Session) ensureUI() {
	if s == nil || !s.wantUI {
		return
	}
	s.uiOnce.Do(func() {
		s.startUI()
	})
}

// Close waits for all tasks, stops the UI, restores global output, then runs
// AfterWait hooks. Idempotent (sync.Once). Returns the first task error, else
// the first AfterWait error.
func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		s.err = s.group.Wait()

		if logging.ContextHasLogger(s.group.ctx) {
			logger := logging.GetLogger(s.group.ctx)
			snap := s.group.snapshotRecursive()
			logger.Debug("session: group finished", "remaining_tasks", len(snap), "err", s.err)
		}

		s.teardownUI()

		s.mu.Lock()
		hooks := make([]func() error, len(s.after))
		copy(hooks, s.after)
		s.after = nil
		s.mu.Unlock()
		for _, fn := range hooks {
			if err := fn(); err != nil && s.err == nil {
				s.err = err
			}
		}
	})
	return s.err
}

func (s *Session) startUI() {
	g := s.group
	model := newBubbleModel(g)
	// Do not bind tea to g.ctx: error cancellation would kill the program with
	// "context canceled" before Close can prefer the task error. Lifetime is
	// owned by Close via Quit.
	prog := tea.NewProgram(model, tea.WithOutput(os.Stderr))

	g.setUsingBubbleTea(true)
	g.SetLogHandler(func(taskName, msg string) {
		prog.Printf("%s", msg)
		// Refresh is best-effort; after Quit, Send must not be used (can stall).
		prog.Send(refreshMsg{})
	})

	s.out = newOutputEnv(prog)
	s.prog = prog
	s.uiDone = make(chan struct{})
	go func() {
		defer close(s.uiDone)
		_, s.uiErr = prog.Run()
	}()
}

// teardownUI restores globals first (so drains never block on tea), then quits
// the program and joins the UI goroutine. No-op if UI never started.
func (s *Session) teardownUI() {
	if s.prog == nil {
		return
	}
	// Restore real stderr/slog before touching tea — avoids pipe drain sitting
	// on prog.Printf while the event loop is stopping.
	if s.out != nil {
		s.out.restore()
		s.out = nil
	}
	s.group.SetLogHandler(nil)
	s.group.setUsingBubbleTea(false)

	s.prog.Quit()
	<-s.uiDone
	s.prog = nil
}

// sessionForGroup finds a Session whose root group is g (pointer equality),
// used by legacy Run/RunBubbleTea shims.
func sessionForGroup(g *Group) *Session {
	if g == nil {
		return nil
	}
	return SessionFrom(g.ctx)
}
