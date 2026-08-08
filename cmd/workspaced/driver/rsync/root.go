package rsync

import (
	"workspaced/pkg/driver/rsync"

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
			// Work is scheduled on the session group inside Sync; PostRun Close waits.
			return rsync.Sync(c.Context(), src, dst, opts)
		},
	}

	cmd.Flags().StringArrayVarP(&excludes, "exclude", "e", nil, "Exclude pattern (repeatable)")
	cmd.Flags().BoolVar(&skipPermissions, "no-perms", false, "Do not preserve permissions (like rsync --no-perms)")
	return cmd
}
