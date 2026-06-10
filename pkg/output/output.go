// Package output provides a terminal progress rendering system integrated
// with taskgroup.Group. It detects whether the terminal supports interactive
// output and chooses between an APT/Gradle-style progress UI (sticky progress
// bars at the bottom, logs scrolling above) or plain text logging.
package output

import (
	"io"
	"os"

	"golang.org/x/term"

	"workspaced/pkg/taskgroup"
)

// Renderer consumes taskgroup.Group snapshots and renders progress.
type Renderer interface {
	// Run renders progress until the group is done or ctx is cancelled.
	// It blocks until rendering is complete.
	Run(g *taskgroup.Group) error
}

// Auto detects terminal capabilities and returns the appropriate renderer.
// If w is a terminal and not dumb, returns an interactive renderer.
// Otherwise returns a plain-text renderer.
func Auto(w *os.File) Renderer {
	if isInteractive(w) {
		return NewInteractive(w)
	}
	return NewPlain(w)
}

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
	return term.IsTerminal(int(w.Fd()))
}

// WriterRenderer wraps a Renderer to expose the underlying writer
// for code that needs to write logs outside the taskgroup system.
type WriterRenderer interface {
	Renderer
	Writer() io.Writer
}
