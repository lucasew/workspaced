// Package cmdregistry provides the CommandRegistry helper used to wire up
// Cobra subcommands in a modular way across many small packages under
// cmd/workspaced/* without creating import cycles.
//
// Each subcommand group (e.g. driver/audio, tool, home/apply) defines a
// GetCommand() function that returns its *cobra.Command (after attaching its
// own children via its local Registry.FillCommands).
//
// The root command and the generated preludes use this to assemble the full tree.
package cmdregistry

import (
	"context"

	"github.com/spf13/cobra"
)

// RegisterFunc is a function type that modifies a cobra.Command.
// Implementations typically add subcommands or flags to the provided parent command.
type RegisterFunc func(*cobra.Command)

// RunE is a context-only command handler (no positional args).
type RunE func(ctx context.Context) error

// CommandRegistry is a helper to aggregate command builders.
// It allows different packages to register their subcommands independently,
// facilitating a modular CLI structure without cyclic dependencies.
type CommandRegistry struct {
	builders []RegisterFunc
}

// Register adds a new builder function to the registry.
// The provided function 'f' will be executed later when GetCommand is invoked.
func (r *CommandRegistry) Register(f RegisterFunc) {
	r.builders = append(r.builders, f)
}

// Add registers a leaf subcommand whose handler only needs the command context.
func (r *CommandRegistry) Add(use, short string, run RunE) {
	r.AddCmd(&cobra.Command{Use: use, Short: short}, run)
}

// AddWithAliases is like Add but attaches command aliases.
func (r *CommandRegistry) AddWithAliases(use, short string, aliases []string, run RunE) {
	r.AddCmd(&cobra.Command{Use: use, Short: short, Aliases: aliases}, run)
}

// AddCmd registers cmd with RunE wired to run(c.Context()).
// Flags, Args, and other cobra.Command fields may be set on cmd beforehand.
func (r *CommandRegistry) AddCmd(cmd *cobra.Command, run RunE) {
	r.Register(func(parent *cobra.Command) {
		cmd.RunE = func(c *cobra.Command, _ []string) error {
			return run(c.Context())
		}
		parent.AddCommand(cmd)
	})
}

func (r *CommandRegistry) FromGetter(cmd func() *cobra.Command) {
	r.Register(func(c *cobra.Command) {
		c.AddCommand(cmd())
	})
}

// FillCommands applies all registered builder functions to the base command.
// It iterates through all registered functions, allowing them to attach their
// respective subcommands to the 'base' command.
func (r *CommandRegistry) FillCommands(base *cobra.Command) *cobra.Command {
	for _, build := range r.builders {
		build(base)
	}
	return base
}
