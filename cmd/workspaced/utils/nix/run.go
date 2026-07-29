package nix

import (
	"context"

	"github.com/lucasew/workspaced/internal/nix"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		cmd := &cobra.Command{
			Use:                "run <ref> [args...]",
			Short:              "Builds a package with RAM caching and runs it locally",
			Args:               cobra.MinimumNArgs(1),
			DisableFlagParsing: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				return runFlakeRef(cmd.Context(), args, func(ctx context.Context, flakeRef string) (string, error) {
					return nix.Build(ctx, flakeRef, true)
				})
			},
		}
		parent.AddCommand(cmd)
	})
}
