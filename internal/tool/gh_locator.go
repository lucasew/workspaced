package tool

import (
	"context"

	"github.com/lucasew/workspaced/internal/githubutil"
)

// Register a fallback so githubutil.Token can ensure github:cli/cli when `gh`
// is not on PATH. Absolute path avoids PATH shims that re-enter workspaced.
//
// Init is safe: locator runs only after all package inits (first Token call).
func init() {
	githubutil.SetGHLocator(func(ctx context.Context) (string, error) {
		m, err := NewManager()
		if err != nil {
			return "", err
		}
		return m.EnsureInstalled(ctx, "github:cli/cli", "gh")
	})
}
