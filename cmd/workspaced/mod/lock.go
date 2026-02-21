package mod

import (
	"fmt"
	"path/filepath"
	"workspaced/pkg/config"
	"workspaced/pkg/module"

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
	root, err := resolveRepoRoot()
	if err != nil {
		return fmt.Errorf("failed to detect repo root: %w", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	modPath := filepath.Join(root, "workspaced.mod.toml")
	sumPath := filepath.Join(root, "workspaced.sum.toml")
	modulesBaseDir := filepath.Join(root, "modules")

	modFile, err := module.LoadModFile(modPath)
	if err != nil {
		return err
	}
	entries, err := module.BuildLockEntries(cfg, modFile, modulesBaseDir)
	if err != nil {
		return err
	}
	if err := module.WriteSumFile(sumPath, &module.SumFile{Modules: entries}); err != nil {
		return err
	}

	cmd.Printf("wrote %s (%d modules)\n", sumPath, len(entries))
	return nil
}
