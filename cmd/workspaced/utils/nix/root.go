package nix

import (
	"errors"
	"github.com/lucasew/workspaced/internal/cmdregistry"

	"github.com/spf13/cobra"
)

var (
	ErrNoFlakeRef    = errors.New("no flake reference provided")
	ErrNoBinaryFound = errors.New("no binary found")
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nix",
		Short: "Nix operations",
	}
	return Registry.FillCommands(cmd)
}
