package tool

import (
	"context"

	"workspaced/pkg/taskgroup"
	"workspaced/pkg/tool"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		c.AddCommand(&cobra.Command{
			Use:   "install <tool-spec>",
			Short: "Install a tool",
			Long: `Install a tool from a provider.

Tool spec format:
  provider:package@version  (full spec)
  provider:package          (uses latest version)
  package@version           (uses registry provider for curated short names)
  package                   (uses registry provider + latest; for curated short names e.g. ripgrep, uv)

Bare names (no provider:) default to the registry provider (curated github tools).
For mise-managed tools (e.g. go, node) or direct github use 'mise:' or 'github:'.`,
			Example: `  workspaced tool install github:denoland/deno@1.40.0
  workspaced tool install ripgrep@14.0.0
  workspaced tool install uv
  workspaced tool install mise:go@latest`,
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				manager, err := tool.NewManager()
				if err != nil {
					return err
				}

				spec := args[0]
				g := taskgroup.MustFromContext(cmd.Context())
				g.Go("tool:install:"+spec, taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
					s.Update("installing " + spec)
					return manager.Install(ctx, spec)
				})
				return nil
			},
		})
	})

}
