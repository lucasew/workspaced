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
		if km.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tickMsg, refreshMsg:
		if m.group == nil {
			return m, m.tick()
		}
		snap := m.group.snapshotRecursive()

		// Drop anything we marked gone last tick (extra frame for 100% was optional;
		// we now remove finished tasks immediately below — this clears stragglers).
		for id := range m.finishedToRemove {
			m.dropBar(id)
		}
		m.finishedToRemove = make(map[string]struct{})

		// Rebuild visible set from live Running tasks with a determinate total.
		// Completed/failed tasks are not shown (they prune from TaskCollection
		// quickly; keeping them in the model made bars linger 100ms+ and stack up).
		seen := make(map[string]struct{}, len(snap))
		for _, t := range snap {
			id := t.ID
			if t.State != Running || t.Total <= 0 {
				continue
			}
			seen[id] = struct{}{}
			pct := float64(t.Current) / float64(t.Total)
			m.percents[id] = pct
			m.statuses[id] = t.Message
			if _, ok := m.pools[id]; !ok {
				m.pools[id] = t.Pool
				m.names[id] = t.Name
				m.order = append(m.order, id)
			}
		}
		// Remove bars for tasks no longer running (finished, pruned, or no total).
		for id := range m.percents {
			if _, ok := seen[id]; !ok {
				m.dropBar(id)
			}
		}
		// Compact order so it does not grow without bound across 10k map items.
		if len(m.order) > len(m.percents)+8 {
			m.compactOrder()
		}
		return m, m.tick()
	}
	return m, nil
}

func (m *bubbleModel) dropBar(id string) {
	delete(m.statuses, id)
	delete(m.percents, id)
	delete(m.pools, id)
	delete(m.names, id)
	delete(m.finalized, id)
}

func (m *bubbleModel) compactOrder() {
	out := m.order[:0]
	for _, id := range m.order {
		if _, ok := m.percents[id]; ok {
			out = append(out, id)
		}
	}
	m.order = out
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

// RunBubbleTea is a compatibility alias for Run (session Close when present).
// Prefer scheduling only and letting PersistentPostRun Session.Close finish work.
func (g *Group) RunBubbleTea() error {
	return Run(g)
}
