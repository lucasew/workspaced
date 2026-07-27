package clipboard

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"strings"

	dapi "github.com/lucasew/workspaced/pkg/api"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/logging"
)

// WriteImageViaCmd PNG-encodes img and feeds it to name with args on stdin.
func WriteImageViaCmd(ctx context.Context, img image.Image, name string, args ...string) error {
	if !execdriver.IsBinaryAvailable(ctx, name) {
		return fmt.Errorf("%w: %s", dapi.ErrBinaryNotFound, name)
	}
	pr, pw := io.Pipe()
	go func() {
		if err := png.Encode(pw, img); err != nil {
			logging.ReportError(ctx, err)
		}
		logging.Close(ctx, pw)
	}()
	cmd := execdriver.MustRun(ctx, name, args...)
	cmd.Stdin = pr
	return cmd.Run()
}

// WriteTextViaCmd feeds text to name with args on stdin.
func WriteTextViaCmd(ctx context.Context, text, name string, args ...string) error {
	if !execdriver.IsBinaryAvailable(ctx, name) {
		return fmt.Errorf("%w: %s", dapi.ErrBinaryNotFound, name)
	}
	cmd := execdriver.MustRun(ctx, name, args...)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
