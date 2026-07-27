package demo

import (
	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		cmd := &cobra.Command{
			Use:   "debug",
			Short: "Debug flag passing",
			RunE: func(cmd *cobra.Command, args []string) error {
				testFlag, err := cmd.Flags().GetString("test")
				if err != nil {
					return err
				}
				cmd.Printf("test flag value: %s\n", testFlag)
				cmd.Printf("args: %v\n", args)
				return nil
			},
		}
		cmd.Flags().String("test", "default", "a test flag")
		parent.AddCommand(cmd)
	})
}
