package mod

import (
	"context"
	"fmt"
	"workspaced/pkg/config"
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

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := ws.EnsureFiles(); err != nil {
		return err
	}

	mod, err := modfile.LoadModFile(ws.ModPath())
	if err != nil {
		return err
	}
	entries, err := modfile.BuildLockEntries(cfg, mod, ws.ModulesBaseDir())
	if err != nil {
		return err
	}
	if err := modfile.WriteSumFile(ws.SumPath(), &modfile.SumFile{Modules: entries}); err != nil {
		return err
	}

	cmd.Printf("wrote %s (%d modules)\n", ws.SumPath(), len(entries))
	return nil
}
