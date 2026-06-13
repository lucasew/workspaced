package nix

import (
	"errors"
	"workspaced/pkg/cmdregistry"

	"github.com/spf13/cobra"
)

var (
	errNoFlakeRef    = errors.New("no flake reference provided")
	errNoBinaryFound = errors.New("no binary found")
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nix",
		Short: "Nix operations",
	}
	return Registry.FillCommands(cmd)
}
