package output

import (
	"os"

	"workspaced/pkg/taskgroup"
)

// BubbleTeaRenderer is a compatibility wrapper around session-backed progress UI.
// Prefer scheduling tasks only; PersistentPostRun Session.Close finishes work.
type BubbleTeaRenderer struct {
	w *os.File
}

func NewBubbleTeaRenderer(w *os.File) Renderer {
	return &BubbleTeaRenderer{w: w}
}

func (r *BubbleTeaRenderer) Run(g *taskgroup.Group) error {
	return taskgroup.Run(g)
}

func Auto(w *os.File) Renderer {
	return NewBubbleTeaRenderer(w)
}
