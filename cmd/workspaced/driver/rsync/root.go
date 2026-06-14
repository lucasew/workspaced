package rsync

import (
	"workspaced/pkg/driver/rsync"
	"workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	var excludes []string
	var skipPermissions bool

	cmd := &cobra.Command{
		Use:   "rsync <src> <dst>",
		Short: "Run a sync through the selected rsync driver (native rsync by default, gokrazy/rsync as fallback)",
		Long: `Invoke the rsync driver abstraction.

This lets you exercise (and test) the currently chosen rsync implementation
as configured by weights in workspaced.cue under the "rsync.Driver" interface.

Native rsync is preferred when the binary is present; otherwise the pure-Go
implementation from github.com/gokrazy/rsync is used automatically.`,
		Args: cobra.ExactArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			src := args[0]
			dst := args[1]
			opts := rsync.Options{
				Excludes:        excludes,
				SkipPermissions: skipPermissions,
				Output:          c.OutOrStdout(),
			}

			ctx := c.Context()
			g := taskgroup.MustFromContext(ctx)

			// Schedule the actual work via the rsync package (which will create
			// the properly-named "rsync:..." task internally via RunWithTaskGroup).
			// We launch it in a goroutine so we can start the group renderer
			// (taskgroup.Run) concurrently to get live progress bars.
			errCh := make(chan error, 1)
			go func() {
				errCh <- rsync.Sync(ctx, src, dst, opts)
			}()

			if runErr := taskgroup.Run(g); runErr != nil {
				// Prefer the actual rsync error if present.
				if err := <-errCh; err != nil {
					return err
				}
				return runErr
			}
			return <-errCh
		},
	}

	cmd.Flags().StringArrayVarP(&excludes, "exclude", "e", nil, "Exclude pattern (repeatable)")
	cmd.Flags().BoolVar(&skipPermissions, "no-perms", false, "Do not preserve permissions (like rsync --no-perms)")

	return cmd
}
