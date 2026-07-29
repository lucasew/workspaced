package tool

import (
	"context"
	"fmt"

	"github.com/lucasew/workspaced/internal/tool"

	parsespec "github.com/lucasew/workspaced/internal/parse/spec"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		c.AddCommand(&cobra.Command{
			Use:   "versions <tool-spec>",
			Short: "List available versions for a tool ref from upstream",
			Long: `List versions reported by the backend for the given tool spec.

The first version printed is the one considered "latest" by the backend.
This works for curated short names (resolved via registry), github:owner/repo, mise:xxx etc.
No installation is performed.`,
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				versions, err := listVersions(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				for _, v := range versions {
					fmt.Fprintln(cmd.OutOrStdout(), v)
				}
				return nil
			},
		})
	})
}

// listVersions resolves a tool-spec and returns versions from the backend.
func listVersions(ctx context.Context, specStr string) ([]string, error) {
	spec, err := parsespec.Parse(specStr)
	if err != nil {
		return nil, err
	}
	p, err := tool.Get(spec.Provider)
	if err != nil {
		return nil, err
	}
	t, err := p.Tool(spec.Package)
	if err != nil {
		return nil, err
	}
	return t.ListVersions(ctx)
}
