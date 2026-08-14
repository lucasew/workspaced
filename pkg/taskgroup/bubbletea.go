package taskgroup

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Run waits for the group's session to finish (tasks + UI teardown), or for
// the group alone when no Session is attached (tests / New without Enter).
//
// Prefer relying on Session.Close from PersistentPostRun; Run remains for
// early teardown (tool with) and legacy callers.
func Run(g *Group) error {
	if g == nil {
		return nil
	}
	if s := sessionForGroup(g); s != nil {
		return s.Close()
	}
	return g.Wait()
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

// barEntry is one visible progress row, keyed by the task's internal UUIDv7.
// Title is the human description from Go(); subtitle is Status.Update text.
type barEntry struct {
	title    string
	subtitle string
	pool     PoolKind
	percent  float64
}

type bubbleModel struct {
	group *Group
	// bars is keyed by task ID (UUIDv7 from Group.Go). Descriptions are not
	// unique, so display state must never use title as a map key.
	bars  map[string]barEntry
	order []string // task IDs in first-seen order (stable row layout)
	live  *liveHub
}

func newBubbleModel(g *Group) bubbleModel {
	return bubbleModel{
		group: g,
		bars:  make(map[string]barEntry),
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
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tickMsg, refreshMsg:
		if m.group == nil {
			return m, m.tick()
		}
		m.syncFromSnapshot(m.group.snapshotRecursive())
		return m, m.tick()
	}
	return m, nil
}

// syncFromSnapshot rebuilds visible bars from running tasks that report a
// determinate total. Keys are always TaskState.ID (internal UUID).
func (m *bubbleModel) syncFromSnapshot(snap []TaskState) {
	seen := make(map[string]struct{}, len(snap))
	for _, t := range snap {
		if t.State != Running || t.Total <= 0 {
			continue
		}
		id := t.ID
		seen[id] = struct{}{}
		pct := float64(t.Current) / float64(t.Total)
		subtitle := t.Message
		if subtitle == "" {
			subtitle = "running"
		}
		title := t.Name
		if title == "" {
			title = id
		}
		if prev, ok := m.bars[id]; ok {
			prev.subtitle = subtitle
			prev.percent = pct
			// Title/pool are fixed at Go(); keep them from first insert.
			m.bars[id] = prev
			continue
		}
		m.bars[id] = barEntry{
			title:    title,
			subtitle: subtitle,
			pool:     t.Pool,
			percent:  pct,
		}
		m.order = append(m.order, id)
	}
	for id := range m.bars {
		if _, ok := seen[id]; !ok {
			m.dropBar(id)
		}
	}
	if len(m.order) > len(m.bars)+8 {
		m.compactOrder()
	}
}

func (m *bubbleModel) dropBar(id string) {
	delete(m.bars, id)
}

func (m *bubbleModel) compactOrder() {
	out := m.order[:0]
	for _, id := range m.order {
		if _, ok := m.bars[id]; ok {
			out = append(out, id)
		}
	}
	m.order = out
}

func (m bubbleModel) View() (view tea.View) {
	view.KeyboardEnhancements = tea.KeyboardEnhancements{}
	view.AltScreen = false
	view.MouseMode = tea.MouseModeNone
	live := m.live.snapshot()
	if len(m.bars) == 0 && len(live) == 0 {
		view.SetContent("")
		return
	}

	var buf bytes.Buffer
	for _, row := range live {
		buf.WriteString(clipRunes(row, 120))
		buf.WriteByte('\n')
	}
	for _, id := range m.order {
		b, ok := m.bars[id]
		if !ok {
			continue
		}
		buf.WriteString(formatBarLine(b, 30))
		buf.WriteByte('\n')
	}
	view.SetContent(buf.String())
	return
}

// formatBarLine renders one row: ICON BAR title: subtitle
// Example: "🌐 [===============---------------] bundle.tar.gz: 5.0 MiB / 10.0 MiB"
// Subtitle should be detail only (sizes, counts, phase) — not a repeat of title
// or a percent already implied by the bar.
func formatBarLine(b barEntry, barWidth int) string {
	return fmt.Sprintf("%s %s %s: %s",
		poolEmoji(b.pool),
		plainBar(b.percent, barWidth),
		b.title,
		b.subtitle,
	)
}

// poolEmoji returns a short emoji based on the task's PoolKind so users can
// distinguish Control / IO / CPU / Internet work at a glance.
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

// plainBar renders a classic ASCII progress bar. Fixed width so bars column-
// align when stacked (ICON is one emoji + space, then the bar).
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

func clipRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	n := 0
	for i := range s {
		if n == max {
			return s[:i]
		}
		n++
	}
	return s
}

// RunBubbleTea is a compatibility alias for Run (session Close when present).
// Prefer scheduling only and letting PersistentPostRun Session.Close finish work.
func (g *Group) RunBubbleTea() error {
	return Run(g)
}
