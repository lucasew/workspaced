package taskgroup

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/progress"
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
func isInteractiveTerminal() bool {
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
	progress map[string]progress.Model
	statuses map[string]string
}

func newBubbleModel(g *Group) bubbleModel {
	return bubbleModel{
		group:    g,
		progress: make(map[string]progress.Model),
		statuses: make(map[string]string),
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
			if t.State == Running {
				if _, ok := m.progress[t.Name]; !ok {
					m.progress[t.Name] = progress.New(progress.WithDefaultGradient())
				}
				p := m.progress[t.Name]
				if t.Total > 0 {
					pct := float64(t.Current) / float64(t.Total)
					p.SetPercent(pct)
					m.progress[t.Name] = p
				}
				m.statuses[t.Name] = t.Message
			} else {
				delete(m.progress, t.Name)
				delete(m.statuses, t.Name)
			}
		}
		if allDone && len(snap) > 0 {
			return m, tea.Quit
		}
		return m, m.tick()
	}

	// Forward animation ticks etc to the progress bubbles.
	for name, p := range m.progress {
		pi, cmd := p.Update(msg)
		m.progress[name] = pi.(progress.Model)
		_ = cmd
	}
	return m, nil
}

func (m bubbleModel) View() string {
	s := ""
	for name, p := range m.progress {
		st := m.statuses[name]
		if st == "" {
			st = "running"
		}
		s += fmt.Sprintf("%s: %s %s\n", name, p.View(), st)
	}
	return s
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
	prog := tea.NewProgram(model, tea.WithOutput(os.Stderr))

	// Wire logs from tasks (via the onLog hook that the recorder calls)
	// to go through prog.Printf. This makes the log writer be the file
	// that tea uses under the hood, producing natural "print log then bar below".
	g.SetLogHandler(func(taskName, msg string) {
		// Emit using standard slog text formatting (same as outside groups).
		rec := slog.NewRecord(time.Now(), slog.LevelInfo, msg, 0)
		rec.AddAttrs(slog.String("task", taskName))
		var buf bytes.Buffer
		h := slog.NewTextHandler(&buf, &slog.HandlerOptions{})
		_ = h.Handle(context.Background(), rec)
		prog.Printf("%s", buf.String())
		prog.Send(refreshMsg{})
	})

	g.setUsingBubbleTea(true)
	defer g.setUsingBubbleTea(false)

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
