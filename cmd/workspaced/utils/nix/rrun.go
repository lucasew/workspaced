package nix

import (
	"context"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/nix"
)

type Rrun struct {
	ref  cmd.StringArg
	dash *cmd.Dash
	rest []cmd.StringArg
}

func (Rrun) Description() string { return "Builds a package remotely and runs it locally" }

func (r *Rrun) Run(ctx context.Context) error {
	return runFlakeRef(ctx, r.ref.Value(), cmd.Values(r.rest), func(ctx context.Context, flakeRef string) (string, error) {
		return nix.RemoteBuild(ctx, flakeRef, "", true)
	})
}
