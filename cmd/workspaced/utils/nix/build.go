package nix

import (
	"context"
	"fmt"
	"os"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/nix"
)

type Build struct {
	NoCache cmd.Flag `long:"no-cache" help:"Disable RAM cache"`
	ref     cmd.StringArg
}

func (Build) Description() string { return "Build a Nix flake reference with RAM caching" }

func (b *Build) Run(ctx context.Context) error {
	path, err := nix.Build(ctx, b.ref.Value(), !b.NoCache.Value())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, path)
	return err
}
