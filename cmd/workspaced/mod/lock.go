package mod

import (
	"context"
	"workspaced/pkg/modfile"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		c.AddCommand(&cobra.Command{
			Use:   "lock",
			Short: "Refresh workspaced.sum.toml for enabled modules",
			RunE:  runModLock,
		})
	})
}

func runModLock(cmd *cobra.Command, args []string) error {
	ws, err := modfile.DetectWorkspace(context.Background(), "")
	if err != nil {
		return err
	}
	result, err := modfile.GenerateLock(context.Background(), ws)
	if err != nil {
		return err
	}
	cmd.Printf("wrote %s (%d sources, %d modules)\n", ws.SumPath(), result.Sources, result.Modules)
	return nil
}
