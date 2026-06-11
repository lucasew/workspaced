package taskgroup

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	statuses map[string]string
	percents map[string]float64
}

func newBubbleModel(g *Group) bubbleModel {
	return bubbleModel{
		group:    g,
		statuses: make(map[string]string),
		percents: make(map[string]float64),
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
	switch msg.(type) {
	case tea.KeyMsg:
		km := msg.(tea.KeyMsg)
		if km.String() == "q" || km.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tickMsg, refreshMsg:
		if m.group == nil {
			return m, m.tick()
		}
		snap := m.group.Snapshot()
		allDone := true
		for _, t := range snap {
			if t.State != Done && t.State != Failed {
				allDone = false
			}

			// Capture the latest progress values from the snapshot for this
			// task name. We do this for both Running and just-completed
			// tasks so that a final Progress(total, total) written right
			// before the task fn returns is guaranteed to be seen and
			// rendered at 100%.
			if t.Total > 0 {
				pct := float64(t.Current) / float64(t.Total)
				m.percents[t.Name] = pct
				m.statuses[t.Name] = t.Message
			} else {
				m.percents[t.Name] = 0
			}

			if t.State != Running {
				// Keep entries for tasks that reached 100% so the bar
				// visibly finishes at full width (with the completion
				// message) instead of disappearing at 90% or 99%.
				if lastPct, ok := m.percents[t.Name]; ok && lastPct >= 0.999 {
					// keep for final 100% render
				} else {
					delete(m.statuses, t.Name)
					delete(m.percents, t.Name)
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

func (m bubbleModel) View() string {
	if len(m.percents) == 0 {
		return ""
	}

	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	for name := range m.percents {
		st := m.statuses[name]
		if st == "" {
			st = "running"
		}
		bar := plainBar(m.percents[name], 30)
		fmt.Fprintf(tw, "%s:\t%s\t%s\n", name, bar, st)
	}
	tw.Flush()
	return buf.String()
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
	filled := int(pct*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
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
		tea.WithInput(nil),
		tea.WithoutSignalHandler(),
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
		prog.Printf("%s", strings.TrimSpace(msg))
		prog.Send(refreshMsg{})
	})

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
