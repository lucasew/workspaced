package taskgroup

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Run starts a Bubble Tea renderer for the group. It is a convenience wrapper
// around g.RunBubbleTea(). See RunBubbleTea for the opt-in behavior and
// TERM=dumb handling.
func Run(g *Group) error {
	if g == nil {
		return nil
	}
	return g.RunBubbleTea()
}

// isInteractiveTerminal returns false for TERM=dumb, NO_COLOR, CI, or when
// stderr is not a character device. This is the guard so the bubbletea
// kick-in becomes a plain Wait() for non-ttys / CI etc.
//
// For testing the TUI code path in this harness (or CI), you can set
// WORKSPACED_FORCE_TUI=1 to bypass the tty check (the bubbletea branch will
// still run its model even if output is captured).
func isInteractiveTerminal() bool {
	if os.Getenv("WORKSPACED_FORCE_TUI") != "" {
		return true
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("CI") != "" {
		return false
	}
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

type refreshMsg struct{}

type bubbleModel struct {
	group    *Group
	statuses map[string]string   // id (uuid) -> message
	percents map[string]float64  // id -> pct
	pools    map[string]PoolKind // id -> pool
	names    map[string]string   // id -> description (for display)
	order    []string            // ids in first-seen order (for stable bars)

	// finishedToRemove holds ids of tasks whose final 100% frame
	// was just rendered; they will be deleted at the start of the
	// next update so they disappear from the list after showing completion.
	finishedToRemove map[string]struct{}

	// finalized tracks tasks whose completion (final payload) has already
	// been displayed for one frame. We stop re-populating them from
	// future snapshots so they stay removed.
	finalized map[string]struct{}
}

func newBubbleModel(g *Group) bubbleModel {
	return bubbleModel{
		group:            g,
		statuses:         make(map[string]string),
		percents:         make(map[string]float64),
		pools:            make(map[string]PoolKind),
		names:            make(map[string]string),
		order:            nil,
		finishedToRemove: make(map[string]struct{}),
		finalized:        make(map[string]struct{}),
	}
}

type tickMsg time.Time

func (m bubbleModel) Init() tea.Cmd {
	return m.tick()
}

func (m bubbleModel) tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m bubbleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		km := msg
		if km.String() == "q" || km.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tickMsg, refreshMsg:
		if m.group == nil {
			return m, m.tick()
		}
		snap := m.group.snapshotRecursive()
		allDone := true

		// Remove any tasks whose final 100% "payload" (completion frame)
		// was shown in the previous tick. This is what makes finished
		// tasks disappear from the list.
		for id := range m.finishedToRemove {
			delete(m.statuses, id)
			delete(m.percents, id)
		}
		m.finishedToRemove = make(map[string]struct{})

		for _, t := range snap {
			if t.State != Done && t.State != Failed {
				allDone = false
			}

			id := t.ID
			if t.Total > 0 {
				if _, already := m.finalized[id]; already {
					// This task's completion payload was already displayed
					// for one frame. Skip re-populating so it stays removed.
					continue
				}
				pct := float64(t.Current) / float64(t.Total)
				m.percents[id] = pct
				m.statuses[id] = t.Message

				// Record pool, name, and first-seen order the first time we see
				// this task with progress. This gives stable ordering in
				// the progress bar view (tasks don't jump around).
				if _, ok := m.pools[id]; !ok {
					m.pools[id] = t.Pool
					m.names[id] = t.Name
					m.order = append(m.order, id)
				}
			} else {
				m.percents[id] = 0
			}

			if t.State != Running {
				if pct, ok := m.percents[id]; ok && pct >= 0.999 {
					// Task just finished. We captured its final payload
					// (Progress(total,total) + last message) so the 100%
					// bar is visible *this* render. Mark for removal on
					// the next tick.
					m.finishedToRemove[id] = struct{}{}
					m.finalized[id] = struct{}{}
					// Leave in percents/statuses for the current View.
				} else {
					delete(m.statuses, id)
					delete(m.percents, id)
				}
			}
		}
		if allDone && len(snap) > 0 {
			return m, tea.Quit
		}
		return m, m.tick()
	}
	return m, nil
}

func (m bubbleModel) View() (view tea.View) {
	view.KeyboardEnhancements = tea.KeyboardEnhancements{}
	view.AltScreen = false
	view.MouseMode = tea.MouseModeNone
	if len(m.percents) == 0 {
		view.SetContent("")
		return
	}

	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)

	for _, id := range m.order {
		if _, ok := m.percents[id]; !ok {
			continue
		}
		pool := m.pools[id]
		emoji := poolEmoji(pool)

		name := m.names[id]
		if name == "" {
			name = id
		}
		st := m.statuses[id]
		if st == "" {
			st = "running"
		}
		bar := plainBar(m.percents[id], 30)
		fmt.Fprintf(tw, "%s %s:\t%s\t%s\n", emoji, name, bar, st)
	}
	tw.Flush()

	view.SetContent(buf.String())
	return
}

// poolEmoji returns a short emoji prefix based on the task's PoolKind.
// This lets users quickly distinguish Control / IO / CPU / Internet work
// in the progress bar view.
func poolEmoji(p PoolKind) string {
	switch p {
	case Control:
		return "🔧"
	case IO:
		return "💾"
	case CPU:
		return "🧠"
	case Internet:
		return "🌐"
	default:
		return "•"
	}
}

// plainBar renders a dead-simple classic progress bar using only ASCII.
// No gradients, no unicode blocks, no colors, no library magic.
func plainBar(pct float64, width int) string {
	if width <= 0 {
		width = 30
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := min(int(pct*float64(width)+0.5), width)
	return "[" + strings.Repeat("=", filled) + strings.Repeat("-", width-filled) + "]"
}

// RunBubbleTea is the group method that kicks in the bubbletea progress+log
// system for this Group.
//
// It is opt-in only: normal commands (self-update, backup, etc) never call it,
// so bubbletea does not run for them.
//
// If the terminal is "dumb" (TERM=dumb, NO_COLOR, CI, or stderr is not a tty),
// this becomes a no-op that simply waits for the group to finish (plain
// transcript via normal slog from the context loggers inside tasks).
//
// When interactive, it starts a tea.Program (no AltScreen), wires task logs
// (from the context logger inside Go funcs) through prog.Printf so they use
// the same output file as tea and naturally scroll above the bars; after each
// log it sends a refresh so the progress bars are re-rendered below the new
// line ("bar moved down").
//
// Call it from showcase commands after you have done your g.Go(...) scheduling,
// e.g. at the end of a demo RunE: return g.RunBubbleTea()
func (g *Group) RunBubbleTea() error {
	if g == nil {
		return nil
	}
	if !isInteractiveTerminal() {
		// Dumb/non-tty: plain behavior, just wait. No TUI side effects.
		return g.Wait()
	}

	model := newBubbleModel(g)
	// WithInput(nil) + WithoutSignalHandler: we don't need interactive key
	// input for this use (the primary quit is when the group is all done).
	// This also avoids requiring /dev/tty when forcing the TUI path in
	// test harnesses, pipes, or certain CI setups (the guard already
	// prevents entry unless WORKSPACED_FORCE_TUI or a real tty).
	prog := tea.NewProgram(model,
		tea.WithOutput(os.Stderr),
		tea.WithContext(g.ctx),
	)

	// Activate the renderer flag first so that any concurrent logs from
	// already-running (or about-to-run) tasks will skip the normal slog
	// delegate (preventing dups). Then attach the handler (SetLogHandler
	// now also pushes the fn into any pre-existing task Status objects,
	// because tasks capture the onLog at g.Go time and the append closure
	// reads the per-Status value).
	g.setUsingBubbleTea(true)
	defer g.setUsingBubbleTea(false)

	// Wire logs by writing them directly to the same writer we passed to
	// tea.WithOutput (os.Stderr). This advances the terminal cursor past the
	// log line (committing it to scrollback). We then Send(refresh) so the
	// model re-renders the progress bar(s) at the *new* bottom position after
	// the log. This produces the desired "logs over bar, bar moves down on
	// each print" without the cursor compensation that prog.Printf performs
	// (which was causing logs to fight the bar region).
	//
	// The bar rendering + Snapshot polling still goes through the tea program.
	g.SetLogHandler(func(taskName, msg string) {
		prog.Printf("%s", msg)
		prog.Send(refreshMsg{})
	})

	// Install patches for os.Stderr, slog.Default(), and the stdlib log
	// package. Any output they receive while the program is running is
	// line-scanned and emitted via prog.Printf + refresh so it appears
	// in the scrolling transcript above the progress bars. The returned
	// cleanup restores the previous values (like a context manager exit).
	cleanupPatches := installTeaPatches(prog)
	defer cleanupPatches()

	_, err := prog.Run()

	// Clear handler after exit.
	g.SetLogHandler(nil)

	// Surface any task error (the prog just waits for done state; Wait
	// returns the first error if any). Demos often ignore it to match
	// previous behavior where failure was observed in PostRun.
	if werr := g.Wait(); werr != nil {
		if err == nil {
			err = werr
		}
	}
	return err
}
