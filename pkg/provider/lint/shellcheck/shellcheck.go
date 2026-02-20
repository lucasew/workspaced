package shellcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"workspaced/pkg/provider/lint"
	"workspaced/pkg/tool"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// Provider implements the lint.Linter interface for shell scripts.
// It executes 'shellcheck' using the workspaced tool system.
type Provider struct{}

// New creates a new ShellCheck provider.
func New() lint.Linter {
	return &Provider{}
}

func init() {
	lint.Register(New())
}

func (p *Provider) Name() string {
	return "shellcheck"
}

func (p *Provider) Detect(ctx context.Context, dir string) (bool, error) {
	// Applies if any .sh file exists in the directory or subdirectories
	found := false
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".sh") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		// Ignore errors during detection
		return false, nil
	}
	return found, nil
}

type Issue struct {
	File      string      `json:"file"`
	Line      int         `json:"line"`
	EndLine   int         `json:"endLine"`
	Column    int         `json:"column"`
	EndColumn int         `json:"endColumn"`
	Level     string      `json:"level"`
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Fix       interface{} `json:"fix"`
}

func (p *Provider) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	// Find all .sh files
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".sh") {
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, nil
	}

	// shellcheck args: -f json [files...]
	args := append([]string{"-f", "json"}, files...)

	// Use tool.EnsureAndRun to execute shellcheck.
	cmd, err := tool.EnsureAndRun(ctx, "github:koalaman/shellcheck@v0.10.0", "shellcheck", args...)
	if err != nil {
		slog.Error("failed to setup shellcheck", "err", err)
		return nil, err
	}

	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// We ignore the error from Run() because shellcheck returns non-zero on issues
	_ = cmd.Run()

	output := stdout.Bytes()
	if len(output) == 0 {
		// If no output but stderr has content, likely a real error
		if stderr.Len() > 0 {
			slog.Error("shellcheck execution failed", "stderr", stderr.String())
			return nil, fmt.Errorf("shellcheck failed: %s", stderr.String())
		}
		return nil, nil
	}

	var issues []Issue
	if err := json.Unmarshal(output, &issues); err != nil {
		slog.Error("failed to parse shellcheck output", "err", err, "output", string(output))
		return nil, err
	}

	return p.convertToSarif(issues), nil
}

func (p *Provider) convertToSarif(issues []Issue) *sarif.Run {
	toolURI := "https://github.com/koalaman/shellcheck"

	run := &sarif.Run{
		Tool: sarif.Tool{
			Driver: &sarif.ToolComponent{
				Name:           "shellcheck",
				InformationURI: &toolURI,
			},
		},
		Results: []*sarif.Result{},
	}

	for _, issue := range issues {
		level := "warning" // default
		switch issue.Level {
		case "error":
			level = "error"
		case "info", "style":
			level = "note"
		}

		ruleId := fmt.Sprintf("SC%d", issue.Code)
		msg := issue.Message
		fileURI := issue.File

		line := issue.Line
		endLine := issue.EndLine
		col := issue.Column
		endCol := issue.EndColumn

		result := &sarif.Result{
			RuleID: &ruleId,
			Level:  &level,
			Message: sarif.Message{
				Text: &msg,
			},
			Locations: []*sarif.Location{
				{
					PhysicalLocation: &sarif.PhysicalLocation{
						ArtifactLocation: &sarif.ArtifactLocation{
							URI: &fileURI,
						},
						Region: &sarif.Region{
							StartLine:   &line,
							EndLine:     &endLine,
							StartColumn: &col,
							EndColumn:   &endCol,
						},
					},
				},
			},
		}

		run.Results = append(run.Results, result)
	}
	return run
}
