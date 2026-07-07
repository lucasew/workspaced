package tool

import (
	"fmt"

	"workspaced/internal/tool"

	parsespec "workspaced/internal/parse/spec"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		c.AddCommand(&cobra.Command{
			Use:   "latest <tool-spec>",
			Short: "Print the latest version string for a tool ref",
			Long: `Resolve and print the latest version for the tool ref (no install performed).

Equivalent to the first entry returned by "tool versions <tool-spec>".`,
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				specStr := args[0]
				spec, err := parsespec.Parse(specStr)
				if err != nil {
					return err
				}

				p, err := tool.Get(spec.Provider)
				if err != nil {
					return err
				}
				t, err := p.Tool(spec.Package)
				if err != nil {
					return err
				}

				versions, err := t.ListVersions(cmd.Context())
				if err != nil {
					return err
				}
				if len(versions) == 0 {
					return fmt.Errorf("no versions found for %s", specStr)
				}
				fmt.Fprintln(cmd.OutOrStdout(), versions[0])
				return nil
			},
		})
	})
}
