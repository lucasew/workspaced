package mod

import (
	"workspaced/pkg/modfile"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		c.AddCommand(&cobra.Command{
			Use:   "lock",
			Short: "Refresh workspaced.lock.json for enabled modules",
			RunE:  runModLock,
		})
	})
}

func runModLock(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	ws, err := modfile.DetectWorkspace(ctx, "")
	if err != nil {
		return err
	}
	result, err := modfile.GenerateLock(ctx, ws)
	if err != nil {
		return err
	}
	if result.Changed {
		cmd.Printf("wrote %s (%d sources)\n", ws.SumPath(), result.Sources)
	} else {
		cmd.Printf("%s up to date (%d sources)\n", ws.SumPath(), result.Sources)
	}
	return nil
}
