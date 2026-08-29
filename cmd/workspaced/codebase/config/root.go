package config

import (
	"github.com/lucasew/workspaced/cmd/workspaced/configcmd"

	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	return configcmd.New(configcmd.Options{
		Scope: "codebase",
	})
}
