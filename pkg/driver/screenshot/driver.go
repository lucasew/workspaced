package screenshot

import (
	"context"
	"errors"
	"image"
	api "workspaced/pkg/driver/wm"
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
