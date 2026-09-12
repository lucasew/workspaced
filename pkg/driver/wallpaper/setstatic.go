package wallpaper

import (
	"context"
	"fmt"

	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
)

// SetStaticViaSystemdUnit resolves name, then runs it under the wallpaper-change
// user unit with extra args (e.g. --bg-fill PATH or -i PATH).
func SetStaticViaSystemdUnit(ctx context.Context, name string, args ...string) error {
	bin, err := execdriver.Which(ctx, name)
	if err != nil {
		return err
	}
	cmdArgs := append([]string{"--user", "-u", "wallpaper-change", "--collect", bin}, args...)
	if err := execdriver.MustRun(ctx, "systemd-run", cmdArgs...).Run(); err != nil {
		return fmt.Errorf("can't run %s in systemd unit: %w", name, err)
	}
	return nil
}
