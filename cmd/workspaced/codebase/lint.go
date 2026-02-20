package codebase

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"workspaced/pkg/provider/lint"
	_ "workspaced/pkg/provider/prelude"

	"github.com/owenrumney/go-sarif/v2/sarif"
	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		var format string

		cmd := &cobra.Command{
			Use:   "lint [path]",
			Short: "Run linters on the specified path (defaults to current directory)",
			RunE: func(cmd *cobra.Command, args []string) error {
				path, err := os.Getwd()
				if err != nil {
					return err
				}
				if len(args) > 0 {
					path = args[0]
				}

				// Run analysis using all registered linters
				report, err := lint.RunAll(cmd.Context(), path)
				if err != nil {
					return err
				}

				// Check for CI environment variables to save SARIF report
				saveSarifToCI(report)

				return printReport(report, format)
			},
		}

		cmd.Flags().StringVarP(&format, "format", "f", "table", "Output format (table, sarif)")

		c.AddCommand(cmd)
	})
}

func saveSarifToCI(report *sarif.Report) {
	sarifEnvVars := []string{"MISE_CI_SARIF_OUTPUT_DIR"}
	for _, envVar := range sarifEnvVars {
		if outputDir := os.Getenv(envVar); outputDir != "" {
			// Ensure directory exists
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to create SARIF output directory %s: %v\n", outputDir, err)
				continue
			}

			sarifPath := filepath.Join(outputDir, "lint.sarif")
			file, err := os.Create(sarifPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to create SARIF report file %s: %v\n", sarifPath, err)
				continue
			}

			encoder := json.NewEncoder(file)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(report); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to write SARIF report to %s: %v\n", sarifPath, err)
			}
			file.Close()
		}
	}
}

func printReport(report *sarif.Report, format string) error {
	switch format {
	case "sarif":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	case "table":
		return printTable(report)
	default:
		return fmt.Errorf("unknown format: %s (supported: table, sarif)", format)
	}
}

func printTable(report *sarif.Report) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TOOL\tLEVEL\tFILE:LINE\tMESSAGE")

	for _, run := range report.Runs {
		toolName := run.Tool.Driver.Name

		if run.Tool.Driver.Name != "" {
			toolName = run.Tool.Driver.Name
		} else if run.Tool.Driver.InformationURI != nil {
			toolName = *run.Tool.Driver.InformationURI
		}

		for _, res := range run.Results {
			file := ""
			line := 0
			msg := ""

			if res.Message.Text != nil {
				msg = *res.Message.Text
			}

			if len(res.Locations) > 0 {
				loc := res.Locations[0].PhysicalLocation
				if loc != nil {
					if loc.ArtifactLocation != nil && loc.ArtifactLocation.URI != nil {
						file = *loc.ArtifactLocation.URI
					}
					if loc.Region != nil && loc.Region.StartLine != nil {
						line = *loc.Region.StartLine
					}
				}
			}

			fileLine := file
			if line > 0 {
				fileLine = fmt.Sprintf("%s:%d", file, line)
			}

			level := "unknown"
			if res.Level != nil {
				level = *res.Level
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", toolName, level, fileLine, msg)
		}
	}
	w.Flush()
	return nil
}
