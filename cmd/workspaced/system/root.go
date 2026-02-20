package system

import (
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "System apply tools. To be implemented.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	return cmd

}
