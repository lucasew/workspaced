package main

import (
	"log/slog"
	"os"
	"workspaced/cmd/workspaced/codebase"
	"workspaced/cmd/workspaced/daemon"
	"workspaced/cmd/workspaced/dispatch"
	"workspaced/cmd/workspaced/doctor"
	"workspaced/cmd/workspaced/history"
	initcmd "workspaced/cmd/workspaced/init"
	"workspaced/cmd/workspaced/input"
	"workspaced/cmd/workspaced/is"
	"workspaced/cmd/workspaced/open"
	"workspaced/cmd/workspaced/selfinstall"
	"workspaced/cmd/workspaced/selfupdate"
	"workspaced/cmd/workspaced/state"
	"workspaced/cmd/workspaced/svc"
	"workspaced/cmd/workspaced/system"
	toolcmd "workspaced/cmd/workspaced/tool"
	"workspaced/pkg/config"
	"workspaced/pkg/shellgen"
	"workspaced/pkg/version"

	"github.com/spf13/cobra"
)

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

	// Main Command Groups
	cmd.AddCommand(input.GetCommand())
	cmd.AddCommand(open.GetCommand())
	cmd.AddCommand(system.GetCommand())
	cmd.AddCommand(state.GetCommand())
	cmd.AddCommand(history.GetCommand())
	cmd.AddCommand(is.GetCommand())
	cmd.AddCommand(svc.GetCommand())
	cmd.AddCommand(toolcmd.GetCommand())
	cmd.AddCommand(codebase.GetCommand())
	cmd.AddCommand(doctor.GetCommand())

	// Installation and setup
	cmd.AddCommand(selfinstall.GetCommand())
	cmd.AddCommand(selfupdate.GetCommand())
	cmd.AddCommand(initcmd.GetCommand())

	cmd.AddCommand(state.GetCommand())
	cmd.AddCommand(history.GetCommand())

	// Daemon and Internal
	cmd.AddCommand(daemon.Command)
	cmd.AddCommand(dispatch.GetCommand()) // Keep hidden or for internal use

	// Set root command for shell completion generation
	shellgen.SetRootCommand(cmd)

	if err := cmd.Execute(); err != nil {
		slog.Error("error", "err", err)
		os.Exit(1)
	}
}
