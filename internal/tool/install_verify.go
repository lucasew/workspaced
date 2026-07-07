package tool

import (
	"context"
	"fmt"
	"os"

	"workspaced/internal/tool/backend"
	"workspaced/internal/tool/checks"
	"workspaced/pkg/logging"
)

// fixAndCheck runs optional InstallFixer.Fix then checks.Run.
// On check failure it removes destDir so a bad tree is not reused.
func fixAndCheck(ctx context.Context, t backend.Tool, destDir string) error {
	if fixer, ok := t.(backend.InstallFixer); ok {
		if err := fixer.Fix(ctx, destDir); err != nil {
			logging.GetLogger(ctx).Warn("post-install fix failed", "err", err, "dir", destDir)
		}
	}
	if err := checks.Run(ctx, destDir, t); err != nil {
		if rmErr := os.RemoveAll(destDir); rmErr != nil {
			logging.GetLogger(ctx).Warn("remove broken install dir failed", "err", rmErr, "dir", destDir)
		}
		return fmt.Errorf("install checks: %w", err)
	}
	return nil
}
