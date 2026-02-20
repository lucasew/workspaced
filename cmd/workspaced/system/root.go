package system

import (
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "To be defined. System apply tools.",
	}
	return cmd

}
