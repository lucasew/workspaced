package codebase

import (
	"os"
	"workspaced/pkg/git"
	"workspaced/pkg/tool"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(
		func(parent *cobra.Command) {
			parent.AddCommand(&cobra.Command{
				Use:   "ci-status [args]",
				Short: "Lazily downloads ci-status and run passing the args",
				RunE: func(cmd *cobra.Command, args []string) error {
					c, err := tool.EnsureAndRun(cmd.Context(), "github:lucasew/ci-status@latest", "ci-status", args...)
					if err != nil {
						return err
					}
					wd, err := os.Getwd()
					if err != nil {
						return err
					}
					c.Dir, err = git.GetRoot(cmd.Context(), wd)
					if err != nil {
						return err
					}
					c.Stdin = os.Stdin
					c.Stdout = os.Stdout
					c.Stderr = os.Stderr
					return c.Run()
				},
			})
		})
}
