package codebase

import (
	"encoding/json"
	"os"

	"workspaced/pkg/provider/lint"
	_ "workspaced/pkg/provider/prelude"

	"github.com/spf13/cobra"
)

func newLintCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "lint [path]",
		Short: "Run linters on the specified path (defaults to current directory)",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}

			// Run analysis using all registered linters
			report, err := lint.RunAll(cmd.Context(), path)
			if err != nil {
				return err
			}

			// Output JSON to stdout
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(report)
		},
	}
}
