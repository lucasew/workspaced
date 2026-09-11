package nix

import (
	"context"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/nix"
)

type Run struct {
	ref  cmd.StringArg
	dash *cmd.Dash
	rest []cmd.StringArg
}

func (Run) Description() string { return "Builds a package with RAM caching and runs it locally" }

func (r *Run) Run(ctx context.Context) error {
	return runFlakeRef(ctx, r.ref.Value(), cmd.Values(r.rest), func(ctx context.Context, flakeRef string) (string, error) {
		return nix.Build(ctx, flakeRef, true)
	})
}
