package screenshot

import (
	"context"
	"errors"
	"fmt"
	"image"
	"strconv"

	api "github.com/lucasew/workspaced/pkg/driver/wm"
)

var (
	ErrSelectionToolNotFound = errors.New("selection tool not found")
	ErrEmptySelection        = errors.New("empty selection")
)

type TargetType int

const (
	TargetAll TargetType = iota
	TargetOutput
	TargetWindow
	TargetSelection
)

type Driver interface {
	Capture(ctx context.Context, rect *api.Rect) (image.Image, error)
	SelectArea(ctx context.Context) (*api.Rect, error)
}

// ParseRectParts turns four decimal integer strings into a Rect.
// Used by grim/slurp and maim/slop SelectArea parsers.
func ParseRectParts(parts []string) (*api.Rect, error) {
	if len(parts) != 4 {
		return nil, fmt.Errorf("expected 4 geometry fields, got %d", len(parts))
	}
	vals := make([]int, 4)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("parse geometry field %d %q: %w", i, p, err)
		}
		vals[i] = n
	}
	return &api.Rect{X: vals[0], Y: vals[1], Width: vals[2], Height: vals[3]}, nil
}
