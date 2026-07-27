package dialog

import (
	"context"
	"fmt"

	"github.com/lucasew/workspaced/internal/executil"
	"github.com/lucasew/workspaced/pkg/driver"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
)

// RequireDisplayBinary reports ErrIncompatible when neither DISPLAY nor
// WAYLAND_DISPLAY is set, or when name is not on PATH.
func RequireDisplayBinary(ctx context.Context, name string) error {
	if executil.GetEnv(ctx, "DISPLAY") == "" && executil.GetEnv(ctx, "WAYLAND_DISPLAY") == "" {
		return fmt.Errorf("%w: neither DISPLAY nor WAYLAND_DISPLAY set", driver.ErrIncompatible)
	}
	return execdriver.RequireBinary(ctx, name)
}
