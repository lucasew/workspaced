package system

import (
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "System apply tools",
	}
	cmd.AddCommand(getApplyCommand())
	return cmd

}
