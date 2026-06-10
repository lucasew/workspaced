package output

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	"workspaced/pkg/taskgroup"
)

// BubbleTeaRenderer uses Bubble Tea to manage the progress bar display.
// It creates a tea.Program (no AltScreen for non-fullscreen), and configures
// the log output for task logs to use the program's Printf (which uses the
// same underlying writer/file that tea uses under the hood).
// This way, logs and the TUI renders are coordinated through the same output.
//
// The model observes the group for progress updates (via Snapshot) and
// re-renders the bars (using bubbles/progress) after logs or updates.
// Logs themselves are printed via prog.Printf in the handler, so they
// "print over" and the bar is re-rendered below (moved down).
//
// No fullscreen. Logs keep standard slog formatting.
type BubbleTeaRenderer struct {
	w *os.File
}

func NewBubbleTeaRenderer(w *os.File) Renderer {
	return &BubbleTeaRenderer{w: w}
}

func (r *BubbleTeaRenderer) Run(g *taskgroup.Group) error {
	model := newBubbleModel(g)
	prog := tea.NewProgram(model, tea.WithOutput(r.w)) // use the provided file/writer

	// Change the log writer for the group to use tea's Printf under the hood.
	// This makes task logs (from context logger or s.Log) go through the
	// same output mechanism as the TUI renders.
	g.SetLogHandler(func(taskName, msg string) {
		// Format as standard slog for consistency with the rest of the app.
		rec := slog.NewRecord(time.Now(), slog.LevelInfo, msg, 0)
		rec.AddAttrs(slog.String("task", taskName))
		var buf bytes.Buffer
		h := slog.NewTextHandler(&buf, &slog.HandlerOptions{})
		h.Handle(context.Background(), rec)
		// Use prog.Printf -- this is the key: logs use the file/writer
		// that tea.Printf uses under the hood.
		prog.Printf("%s", buf.String())
		// Trigger a re-render of the bars so the bar "moves down" after the log.
		prog.Send(refreshMsg{})
	})

	_, err := prog.Run()
	return err
}

type refreshMsg struct{}

type bubbleModel struct {
	group    *taskgroup.Group
	progress map[string]progress.Model
	statuses map[string]string
}

func newBubbleModel(g *taskgroup.Group) bubbleModel {
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
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tickMsg, refreshMsg:
		snap := m.group.Snapshot()
		allDone := true
		for _, t := range snap {
			if t.State != taskgroup.Done && t.State != taskgroup.Failed {
				allDone = false
			}
			if t.State == taskgroup.Running {
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

	// Update progress components
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

// isInteractive kept for compatibility with Auto etc.
func isInteractive(w *os.File) bool {
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("CI") != "" {
		return false
	}
	fi, err := w.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func Auto(w *os.File) Renderer {
	return NewBubbleTeaRenderer(w)
}
