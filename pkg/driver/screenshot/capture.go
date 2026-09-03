package screenshot

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/png"

	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
)

// CaptureViaCmd runs name with args and decodes stdout as an image.
func CaptureViaCmd(ctx context.Context, name string, args ...string) (image.Image, error) {
	out, err := execdriver.MustRun(ctx, name, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", name, err)
	}
	img, _, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		return nil, fmt.Errorf("decode %s output: %w", name, err)
	}
	return img, nil
}
