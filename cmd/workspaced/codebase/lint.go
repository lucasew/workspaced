package codebase

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"text/tabwriter"
	"time"

	"workspaced/pkg/driver"
	httpclientdriver "workspaced/pkg/driver/httpclient"
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

				// Upload SARIF report to GitHub if running in GitHub Actions
				if os.Getenv("GITHUB_ACTIONS") == "true" {
					uploadSarifToGithub(cmd.Context(), report)
				}

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
				slog.Warn("failed to create SARIF output directory", "dir", outputDir, "error", err)
				continue
			}

			sarifPath := filepath.Join(outputDir, "lint.sarif")
			file, err := os.Create(sarifPath)
			if err != nil {
				slog.Warn("failed to create SARIF report file", "path", sarifPath, "error", err)
				continue
			}

			encoder := json.NewEncoder(file)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(report); err != nil {
				slog.Warn("failed to write SARIF report", "path", sarifPath, "error", err)
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
	if _, err := fmt.Fprintln(w, "TOOL\tLEVEL\tFILE:LINE\tMESSAGE"); err != nil {
		return err
	}

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

			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", toolName, level, fileLine, msg); err != nil {
				return err
			}
		}
	}
	return w.Flush()
}

func uploadSarifToGithub(ctx context.Context, report *sarif.Report) {
	repo := os.Getenv("GITHUB_REPOSITORY")
	token := os.Getenv("GITHUB_TOKEN")
	sha := os.Getenv("GITHUB_SHA")
	ref := os.Getenv("GITHUB_REF")
	workflowRunIDStr := os.Getenv("GITHUB_RUN_ID")
	workflowRunAttemptStr := os.Getenv("GITHUB_RUN_ATTEMPT")
	workspace := os.Getenv("GITHUB_WORKSPACE")

	if repo == "" || token == "" || sha == "" || ref == "" {
		slog.Warn("skipping SARIF upload: missing required environment variables (GITHUB_REPOSITORY, GITHUB_TOKEN, GITHUB_SHA, GITHUB_REF)")
		return
	}

	workflowRunID, err := strconv.Atoi(workflowRunIDStr)
	if err != nil {
		slog.Warn("failed to parse GITHUB_RUN_ID", "error", err, "val", workflowRunIDStr)
		// Proceeding with 0 as it's just metadata
	}

	workflowRunAttempt, err := strconv.Atoi(workflowRunAttemptStr)
	if err != nil {
		slog.Warn("failed to parse GITHUB_RUN_ATTEMPT", "error", err, "val", workflowRunAttemptStr)
		// Proceeding with 0 as it's just metadata
	}

	// Serialize SARIF report
	var sarifBuf bytes.Buffer
	if err := json.NewEncoder(&sarifBuf).Encode(report); err != nil {
		slog.Error("failed to encode SARIF report for upload", "error", err)
		return
	}

	// Gzip and Base64 encode
	var gzipBuf bytes.Buffer
	gz := gzip.NewWriter(&gzipBuf)
	if _, err := gz.Write(sarifBuf.Bytes()); err != nil {
		slog.Error("failed to gzip SARIF report", "error", err)
		return
	}
	if err := gz.Close(); err != nil {
		slog.Error("failed to close gzip writer", "error", err)
		return
	}
	encodedSarif := base64.StdEncoding.EncodeToString(gzipBuf.Bytes())

	toolNames := []string{}
	for _, run := range report.Runs {
		if run.Tool.Driver != nil {
			toolNames = append(toolNames, run.Tool.Driver.Name)
		}
	}

	// Construct payload
	payload := map[string]interface{}{
		"commit_oid":           sha,
		"ref":                  ref,
		"analysis_key":         "workspaced-codebase-lint",
		"analysis_name":        "workspaced codebase lint",
		"sarif":                encodedSarif,
		"workflow_run_id":      workflowRunID,
		"workflow_run_attempt": workflowRunAttempt,
		"checkout_uri":         "file://" + workspace,
		"started_at":           time.Now().Format(time.RFC3339),
		"tool_names":           toolNames,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal payload for SARIF upload", "error", err)
		return
	}

	apiURL := os.Getenv("GITHUB_API_URL")
	if apiURL == "" {
		apiURL = "https://api.github.com"
	}
	url := fmt.Sprintf("%s/repos/%s/code-scanning/sarifs", apiURL, repo)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		slog.Error("failed to create request for SARIF upload", "error", err)
		return
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// Use the httpclient driver
	httpClientDriver, err := driver.Get[httpclientdriver.Driver](ctx)
	if err != nil {
		slog.Error("failed to get httpclient driver", "error", err)
		return
	}
	client := httpClientDriver.Client()

	resp, err := client.Do(req)
	if err != nil {
		slog.Error("failed to upload SARIF report", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		slog.Info("successfully uploaded SARIF report to GitHub Code Scanning")
	} else {
		body, _ := io.ReadAll(resp.Body)
		slog.Warn("failed to upload SARIF report", "status", resp.Status, "body", string(body))
	}
}
