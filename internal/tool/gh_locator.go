package tool

import (
	"context"

	"github.com/lucasew/workspaced/internal/githubutil"
)

// Register a fallback so githubutil.Token can ensure lazy_tools.gh when `gh`
// is not on PATH. Absolute path avoids PATH shims that re-enter workspaced.
// Version comes from the workspace lockfile (prelude default: github:cli/cli).
//
// Init is safe: locator runs only after all package inits (first Token call).
func init() {
	githubutil.SetGHLocator(func(ctx context.Context) (string, error) {
		return ResolveLazyTool(ctx, "gh", "gh")
	})
}
