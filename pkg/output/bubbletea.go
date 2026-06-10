package output

import (
	"os"

	"workspaced/pkg/taskgroup"
)

// BubbleTeaRenderer is a compatibility wrapper around the bubbletea renderer
// that lives on the Group (see taskgroup.Group.RunBubbleTea). The actual
// TUI logic, TERM=dumb guard, and prog.Printf log routing are centralized in
// the taskgroup package as part of the core primitive.
type BubbleTeaRenderer struct {
	w *os.File
}

func NewBubbleTeaRenderer(w *os.File) Renderer {
	return &BubbleTeaRenderer{w: w}
}

func (r *BubbleTeaRenderer) Run(g *taskgroup.Group) error {
	// Delegate to the canonical implementation on the Group (the group
	// system owns the bubbletea integration). This also ensures the
	// TERM=dumb guard and opt-in semantics are centralized.
	// (The w field is kept for API compatibility but ignored; the group
	// method always targets os.Stderr for the tea program.)
	return g.RunBubbleTea()
}

func Auto(w *os.File) Renderer {
	return NewBubbleTeaRenderer(w)
}
