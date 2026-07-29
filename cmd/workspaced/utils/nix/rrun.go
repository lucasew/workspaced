package nix

import (
	"context"

	"github.com/lucasew/workspaced/internal/nix"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		cmd := &cobra.Command{
			Use:                "rrun <ref> [args...]",
			Short:              "Builds a package remotely and runs it locally",
			Args:               cobra.MinimumNArgs(1),
			DisableFlagParsing: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				return runFlakeRef(cmd.Context(), args, func(ctx context.Context, flakeRef string) (string, error) {
					return nix.RemoteBuild(ctx, flakeRef, "", true)
				})
			},
		}
		parent.AddCommand(cmd)
	})
}
