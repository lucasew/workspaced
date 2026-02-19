package codebase

import (
	"encoding/json"
	"os"

	"workspaced/pkg/provider/lint"
	"workspaced/pkg/provider/lint/golangci"

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

			// Setup engine with all available linters
			engine := lint.NewEngine(
				golangci.New(),
				// Add more linters here as they are implemented
			)

			// Run analysis
			report, err := engine.RunAll(cmd.Context(), path)
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
