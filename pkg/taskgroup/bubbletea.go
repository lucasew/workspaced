package taskgroup

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// BubbleTeaModel is a Bubble Tea model that observes a Group (the core
// primitive for concurrent work + progress reporting via Status).
// It drives progress bars from Snapshot (using Current/Total and Message
// from Status) and logs from the per-task log buffers (populated when using
// the context logger inside tasks, or via SetLogHandler / Status.Log).
// This is part of the group system, usable by any UI layer (replacing or
// alongside the ANSI renderer in pkg/output).
//
// Cmd entrypoints obtain the group from context (spread from root), schedule
// work on it using the task primitive (g.Go + s.Update/s.Progress + context
// logger), then call Run or use the model.
type BubbleTeaModel struct {
	progress map[string]progress.Model // per-task progress bars, using the group's Status primitive
	logs     []string
	status   string
	done     bool

	group    *Group
	lastLogs map[string]int
	updates  chan tea.Msg
}

var (
	btTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	btLogStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// NewBubbleTeaModel returns a model for the given group.
func NewBubbleTeaModel(g *Group) BubbleTeaModel {
	return BubbleTeaModel{
		progress: make(map[string]progress.Model),
		status:   "running...",
		group:    g,
		lastLogs: make(map[string]int),
		updates:  make(chan tea.Msg, 32),
	}
}

// Run starts a Bubble Tea program for the group (AltScreen).
func Run(g *Group) error {
	m := NewBubbleTeaModel(g)
	// No AltScreen: no fullscreen takeover. The program renders the current
	// bars (from group's Status) and logs (fed by context logger via buffers/handler)
	// inline. Logs via the handler use prog.Printf to go through tea's output.
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}

type (
	btLogMsg      string
	btProgressMsg struct {
		percent float64
		status  string
	}
	btDoneMsg struct{}
)

func (m BubbleTeaModel) Init() tea.Cmd {
	if m.group != nil {
		m.group.SetLogHandler(func(name, msg string) {
			m.updates <- btLogMsg(fmt.Sprintf("[%s] %s", name, msg))
		})
	}
	return tea.Batch(
		m.waitForLogs(),
		m.poll(),
	)
}

func (m BubbleTeaModel) waitForLogs() tea.Cmd {
	return func() tea.Msg {
		return <-m.updates
	}
}

func (m BubbleTeaModel) poll() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		if m.group == nil {
			return nil
		}
		snap := m.group.Snapshot()
		for _, t := range snap {
			// General for any tasks (the primitive). Use Status data for progress.
			if t.State == Running || t.State == Pending {
				if _, ok := m.progress[t.Name]; !ok {
					m.progress[t.Name] = progress.New(progress.WithDefaultGradient())
				}
				p := m.progress[t.Name]
				pct := 0.0
				if t.Total > 0 {
					pct = float64(t.Current) / float64(t.Total)
				}
				p.SetPercent(pct)
				m.progress[t.Name] = p
				last := m.lastLogs[t.Name]
				if len(t.Logs) > last {
					for _, line := range t.Logs[last:] {
						m.updates <- btLogMsg(fmt.Sprintf("[%s] %s", t.Name, line))
					}
					m.lastLogs[t.Name] = len(t.Logs)
				}
				return btProgressMsg{
					percent: pct,
					status:  t.Message,
				}
			}
		}
		// Check if all done
		allDone := true
		for _, t := range snap {
			if t.State != Done && t.State != Failed {
				allDone = false
				break
			}
		}
		if allDone && len(snap) > 0 {
			return btDoneMsg{}
		}
		return nil
	})
}

func (m BubbleTeaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" || msg.String() == "esc" {
			return m, tea.Quit
		}
	case btLogMsg:
		m.logs = append(m.logs, string(msg))
		if len(m.logs) > 12 {
			m.logs = m.logs[len(m.logs)-12:]
		}
		return m, m.waitForLogs()
	case btProgressMsg:
		// The poll already updates the per-task progress map using the group's Status.
		if msg.status != "" {
			m.status = msg.status
		}
		if msg.percent >= 1.0 {
			m.done = true
			m.status = "Done! Press q to quit."
			return m, nil
		}
		return m, m.poll()
	case btDoneMsg:
		m.done = true
		m.status = "All done! Press q to quit."
		return m, nil
	}
	// Forward to all progress components for animation etc.
	for name, p := range m.progress {
		pi, cmd := p.Update(msg)
		m.progress[name] = pi.(progress.Model)
		_ = cmd
	}
	return m, nil
}

func (m BubbleTeaModel) View() string {
	s := btTitleStyle.Render("workspaced taskgroup progress (bubbletea renderer)") + "\n\n"
	// General: show progress bars for tasks using the group's Status data (the primitive).
	for name, p := range m.progress {
		st := m.status
		s += fmt.Sprintf("%s: %s\n", name, st)
		s += p.View() + "\n\n"
	}
	if m.status != "" {
		s += m.status + "\n\n"
	}
	s += "Logs (from taskgroup + context logger):\n"
	for _, l := range m.logs {
		s += "  " + btLogStyle.Render(l) + "\n"
	}
	if !m.done {
		s += "\n" + btLogStyle.Render("press q to quit")
	}
	return s
}
