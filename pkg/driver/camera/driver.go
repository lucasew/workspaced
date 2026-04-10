package camera

import (
	"context"
	"image"
)

// Driver is the entrypoint to the camera framework, allowing discovery of cameras.
type Driver interface {
	// List returns a list of all currently available cameras.
	List(ctx context.Context) ([]Camera, error)
}

// Camera represents an individual camera device.
type Camera interface {
	// ID returns a unique identifier for the camera.
	ID() string
	// Name returns a human-readable name for the camera.
	Name() string
	// Capture takes a single still frame from the camera.
	Capture(ctx context.Context) (image.Image, error)
}
