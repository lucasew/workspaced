package tool

import (
	"context"
	"fmt"

	"workspaced/pkg/taskgroup"
	"workspaced/pkg/tool"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		c.AddCommand(&cobra.Command{
			Use:   "which <tool-spec> <binary>",
			Short: "Print absolute path to a binary inside the (ensured) tool ref",
			Long: `Resolve and print the full on-disk path for <binary> provided by <tool-spec>.

The tool version is ensured first (installing if the version directory is missing or empty),
exactly like the behavior inside "tool with".
Useful from scripts or to locate exact executables.

The absolute path is the command's final output ("verdict") and is written to stdout.
This means 'workspaced tool which <spec> <bin> 2>/dev/null' will still produce the path on stdout.`,
			Args: cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				spec := args[0]
				binary := args[1]

				m, err := tool.NewManager()
				if err != nil {
					return err
				}

				g := taskgroup.MustFromContext(cmd.Context())
				var binPath string
				g.Go("tool:which:"+spec+":"+binary, taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
					s.Update("ensuring " + spec)
					bp, err := m.EnsureInstalled(ctx, spec, binary)
					if err != nil {
						return err
					}
					binPath = bp
					return nil
				})
				if err := taskgroup.Run(g); err != nil {
					return err
				}
				// Final path (the "verdict" for this command) goes to the command's stdout.
				// Using OutOrStdout makes it robust for redirection and scripting
				// (e.g. `workspaced tool which uv uv 2>/dev/null` still yields the path on stdout).
				if binPath != "" {
					fmt.Fprintln(cmd.OutOrStdout(), binPath)
				}
				return nil
			},
		})
	})
}
