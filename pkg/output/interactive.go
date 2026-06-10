package output

import (
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"workspaced/pkg/taskgroup"
)

// interactiveRenderer renders APT/Gradle-style progress:
// - Log lines scroll up in the main terminal area
// - Active task progress bars are sticky at the bottom
// - On completion, the sticky region is cleared
type interactiveRenderer struct {
	w          *os.File
	stickyRows int // number of rows currently occupied by sticky area
}

// NewInteractive returns an interactive terminal progress renderer.
func NewInteractive(w *os.File) Renderer {
	return &interactiveRenderer{w: w}
}

func (r *interactiveRenderer) Run(g *taskgroup.Group) error {
	lastLogCount := map[string]int{}

	// We poll Snapshot for both progress bars and new task logs (diffed via
	// lastLogCount). Task logs are also forwarded to slog via the handler
	// installed by the root command (see cmd/workspaced/root.go).
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		snap := g.Snapshot()
		allDone := true

		// Collect new log lines to print above the sticky area.
		var newLines []string
		for _, t := range snap {
			if t.State != taskgroup.Done && t.State != taskgroup.Failed {
				allDone = false
			}
			prev := lastLogCount[t.Name]
			if len(t.Logs) > prev {
				for _, line := range t.Logs[prev:] {
					newLines = append(newLines, fmt.Sprintf("[%s] %s", t.Name, line))
				}
				lastLogCount[t.Name] = len(t.Logs)
			}
		}

		// Build the active tasks bar lines.
		var activeBars []string
		for _, t := range snap {
			if t.State == taskgroup.Running {
				activeBars = append(activeBars, r.formatProgressLine(t))
			}
		}

		// Render: clear sticky, print new log lines, redraw sticky.
		r.clearSticky()

		if len(newLines) > 0 {
			for _, line := range newLines {
				fmt.Fprintln(r.w, line)
			}
		}

		// Print state transitions (completed/failed) above sticky.
		for _, t := range snap {
			switch t.State {
			case taskgroup.Done:
				key := "done:" + t.Name
				if lastLogCount[key] == 0 {
					fmt.Fprintf(r.w, "\033[32m✓\033[0m %s\n", t.Name)
					lastLogCount[key] = 1
				}
			case taskgroup.Failed:
				key := "fail:" + t.Name
				if lastLogCount[key] == 0 {
					errMsg := "failed"
					if t.Error != nil {
						errMsg = t.Error.Error()
					}
					fmt.Fprintf(r.w, "\033[31m✗\033[0m %s: %s\n", t.Name, errMsg)
					lastLogCount[key] = 1
				}
			}
		}

		// Draw sticky progress bars at the bottom.
		r.drawSticky(activeBars)

		if allDone && len(snap) > 0 {
			r.clearSticky()
			return nil
		}

		// Exit promptly when the group is finalized (Wait called in PostRun,
		// or first error). This prevents hangs for commands that schedule
		// zero tasks on the root group (self-update, many utils, etc).
		if g.Context().Err() != nil {
			r.clearSticky()
			return nil
		}

		select {
		case <-ticker.C:
		case <-g.Context().Done():
			r.clearSticky()
			return nil
		}
	}
}

func (r *interactiveRenderer) clearSticky() {
	if r.stickyRows == 0 {
		return
	}
	// Move cursor up N rows, clear each line.
	for i := 0; i < r.stickyRows; i++ {
		fmt.Fprint(r.w, "\033[A\033[2K")
	}
	r.stickyRows = 0
}

func (r *interactiveRenderer) drawSticky(lines []string) {
	if len(lines) == 0 {
		r.stickyRows = 0
		return
	}
	for _, line := range lines {
		fmt.Fprintln(r.w, line)
	}
	r.stickyRows = len(lines)
}

func (r *interactiveRenderer) termWidth() int {
	w, _, err := term.GetSize(int(r.w.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

func (r *interactiveRenderer) formatProgressLine(t taskgroup.TaskState) string {
	width := r.termWidth()

	// Format: ▶ task-name ━━━━━━━━░░░░ 50% message
	name := t.Name
	if len(name) > 20 {
		name = name[:17] + "..."
	}

	prefix := fmt.Sprintf("\033[36m▶\033[0m %-20s ", name)
	prefixLen := 2 + 1 + 20 + 1 // icon + space + name + space

	msg := t.Message
	if msg == "" {
		msg = "running"
	}

	if t.Total > 0 {
		// Determinate progress bar.
		pct := float64(t.Current) / float64(t.Total)
		if pct > 1 {
			pct = 1
		}

		pctStr := fmt.Sprintf(" %3d%%", int(pct*100))

		// Available space for bar + pct + space + msg
		barWidth := width - prefixLen - len(pctStr) - 1 - len(msg)
		if barWidth < 5 {
			barWidth = 5
		}
		if barWidth > 30 {
			barWidth = 30
		}

		filled := int(float64(barWidth) * pct)
		if filled > barWidth {
			filled = barWidth
		}
		empty := barWidth - filled

		bar := "\033[32m" + strings.Repeat("━", filled) + "\033[90m" + strings.Repeat("━", empty) + "\033[0m"

		line := prefix + bar + pctStr + " " + msg
		return truncate(line, width)
	}

	// Indeterminate — just show message.
	line := prefix + msg
	return truncate(line, width)
}

func truncate(s string, maxWidth int) string {
	// Simple byte-level truncation (ANSI escapes make rune counting tricky,
	// but this is good enough for terminal rendering).
	visible := stripANSI(s)
	if len(visible) <= maxWidth {
		return s
	}
	// Rough truncation — cut the original string.
	// This may cut an ANSI sequence, but terminals handle it gracefully.
	if len(s) > maxWidth+20 { // +20 for ANSI overhead
		return s[:maxWidth+15] + "\033[0m"
	}
	return s
}

func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inEsc := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') {
				inEsc = false
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// LogAbove prints a message above the sticky progress area.
// This is useful for code outside taskgroup that wants to print without
// interfering with the progress display.
func LogAbove(r Renderer, msg string) {
	if ir, ok := r.(*interactiveRenderer); ok {
		ir.clearSticky()
		fmt.Fprintln(ir.w, msg)
		// Sticky will be redrawn on next tick.
		return
	}
	// For plain renderers, just print to stderr as fallback.
	fmt.Fprintln(os.Stderr, msg)
}
