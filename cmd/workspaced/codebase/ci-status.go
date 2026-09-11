package codebase

import (
	"github.com/lucasew/workspaced/internal/git"
	"github.com/lucasew/workspaced/internal/tool"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(
		func(parent *cobra.Command) {
			parent.AddCommand(&cobra.Command{
				Use:   "ci-status [args]",
				Short: "Run ci-status from the workspace lazy_tools pin",
				RunE: func(cmd *cobra.Command, args []string) error {
					c, err := tool.EnsureAndRunLazy(cmd.Context(), "ci_status", "ci-status", args...)
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
