package brightness

import (
	"context"
	"errors"
)

var (
	ErrDeviceNotFound = errors.New("brightness device not found")
)

type Device struct {
	Name       string
	Brightness float64
}

type Driver interface {
	SetBrightness(ctx context.Context, brightness float64) error
	Status(ctx context.Context) (*Device, error)
}
