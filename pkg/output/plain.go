package output

import (
	"fmt"
	"io"
	"strings"
	"time"

	"workspaced/pkg/taskgroup"
)

// plainRenderer prints task progress as simple log lines.
// No ANSI codes, works everywhere: CI, pipes, dumb terminals.
type plainRenderer struct {
	w io.Writer
}

// NewPlain returns a plain-text progress renderer.
func NewPlain(w io.Writer) Renderer {
	return &plainRenderer{w: w}
}

func (r *plainRenderer) Run(g *taskgroup.Group) error {
	// Track what we've already printed to avoid spam.
	// Task logs (from s.Log inside tasks) are accumulated in the snapshot
	// and also forwarded to the app's slog via SetLogHandler (wired in root.go).
	lastState := map[string]taskgroup.State{}
	lastMsg := map[string]string{}
	lastLogCount := map[string]int{}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		snap := g.Snapshot()
		allDone := true

		for _, t := range snap {
			if t.State != taskgroup.Done && t.State != taskgroup.Failed {
				allDone = false
			}

			// Print new log lines.
			prev := lastLogCount[t.Name]
			if len(t.Logs) > prev {
				for _, line := range t.Logs[prev:] {
					fmt.Fprintf(r.w, "[%s] %s\n", t.Name, line)
				}
				lastLogCount[t.Name] = len(t.Logs)
			}

			// Print state transitions.
			if t.State != lastState[t.Name] {
				switch t.State {
				case taskgroup.Running:
					msg := t.Message
					if msg == "" {
						msg = "started"
					}
					fmt.Fprintf(r.w, "[%s] %s\n", t.Name, msg)
				case taskgroup.Done:
					fmt.Fprintf(r.w, "[%s] done\n", t.Name)
				case taskgroup.Failed:
					errMsg := "failed"
					if t.Error != nil {
						errMsg = t.Error.Error()
					}
					fmt.Fprintf(r.w, "[%s] FAILED: %s\n", t.Name, errMsg)
				}
				lastState[t.Name] = t.State
				lastMsg[t.Name] = t.Message
			} else if t.State == taskgroup.Running && t.Message != lastMsg[t.Name] && strings.TrimSpace(t.Message) != "" {
				fmt.Fprintf(r.w, "[%s] %s\n", t.Name, t.Message)
				lastMsg[t.Name] = t.Message
			}
		}

		if allDone && len(snap) > 0 {
			return nil
		}

		// Exit promptly when the group is finalized (Wait called in PostRun,
		// or first error). This prevents hangs for commands that schedule
		// zero tasks on the root group (self-update, many utils, etc).
		if g.Context().Err() != nil {
			return nil
		}

		select {
		case <-ticker.C:
		case <-g.Context().Done():
			return nil
		}
	}
}
