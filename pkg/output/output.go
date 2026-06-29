// Package output provides a terminal progress rendering system integrated
// with taskgroup.Group. It detects whether the terminal supports interactive
// output and chooses between an APT/Gradle-style progress UI (sticky progress
// bars at the bottom, logs scrolling above) or plain text logging.
package output

import (
	"io"

	"workspaced/pkg/taskgroup"
)

// Renderer consumes taskgroup.Group snapshots and renders progress.
type Renderer interface {
	// Run renders progress until the group is done or ctx is cancelled.
	// It blocks until rendering is complete.
	Run(g *taskgroup.Group) error
}

// WriterRenderer wraps a Renderer to expose the underlying writer
// for code that needs to write logs outside the taskgroup system.
type WriterRenderer interface {
	Renderer
	Writer() io.Writer
}

// Note: Auto() and NewBubbleTeaRenderer are thin wrappers. The real
// implementation (including the opt-in Group method, dumb-term guard and
// natural log scrolling) lives in pkg/taskgroup as Session (Enter/Close).
