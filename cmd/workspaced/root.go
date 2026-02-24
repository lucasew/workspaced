package main

import (
	"log/slog"
	"os"

	"workspaced/pkg/config"
	"workspaced/pkg/registry"
	"workspaced/pkg/shellgen"
	"workspaced/pkg/version"

	"github.com/spf13/cobra"
)

var Registry registry.CommandRegistry

func main() {
	// Load config early to set driver weights
	if _, err := config.Load(); err != nil {
		slog.Debug("failed to load config", "error", err)
	}

	var verbose bool

	cmd := &cobra.Command{
		Use:     "workspaced",
		Short:   "workspaced - declarative user environment manager",
		Version: version.GetBuildID(),
		PersistentPreRun: func(c *cobra.Command, args []string) {
			if verbose {
				slog.SetLogLoggerLevel(slog.LevelDebug)
			}
		},
	}

	// Global flags
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug logging")
	Registry.FillCommands(cmd)

	// Set root command for shell completion generation
	shellgen.SetRootCommand(cmd)

	if err := cmd.Execute(); err != nil {
		slog.Error("error", "err", err)
		os.Exit(1)
	}
}
