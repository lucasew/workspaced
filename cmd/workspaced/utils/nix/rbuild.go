package nix

import (
	"context"
	"fmt"
	"os"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/nix"
)

type Rbuild struct {
	Target     cmd.StringArg `short:"t" long:"target" help:"Remote host to build on (default: whiterun)"`
	CopyBack   cmd.Flag      `long:"copy-back" help:"Copy result back to local store" default:"true"`
	NoCopyBack cmd.Flag      `long:"no-copy-back" help:"Do not copy result back to local store"`
	Nom        cmd.Flag      `long:"nom" help:"Use nix-output-monitor (nom)" default:"true"`
	ref        cmd.StringArg
}

func (Rbuild) Description() string { return "Performs a remote build of a Nix flake reference" }

func (r *Rbuild) Run(ctx context.Context) error {
	resultPath, err := nix.RemoteBuild(ctx, r.ref.Value(), r.Target.Value(), r.CopyBack.Value() && !r.NoCopyBack.Value())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, resultPath)
	return err
}
