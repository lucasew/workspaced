package actionlint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"workspaced/internal/checks"
	"workspaced/internal/checks/lint"
	"workspaced/internal/tool"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// check implements the lint.Linter interface for GitHub Actions workflows.
type check struct{}

const actionlintInfoURI = "https://github.com/rhysd/actionlint"

// New creates a new actionlint check.
func New() lint.Linter {
	return &check{}
}

func init() {
	lint.Register(New())
}

func (c *check) Name() string {
	return "actionlint"
}

func (c *check) Detect(ctx context.Context, dir string) error {
	// Applies if .github/workflows exists and is not empty
	workflowsDir := filepath.Join(dir, ".github", "workflows")
	entries, err := os.ReadDir(workflowsDir)
	if errors.Is(err, os.ErrNotExist) {
		return checks.ErrNotApplicable
	}
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return checks.ErrNotApplicable
	}
	return nil
}

type Issue struct {
	Message   string `json:"message"`
	Filepath  string `json:"filepath"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Kind      string `json:"kind"`
	Snippet   string `json:"snippet,omitempty"`
	EndColumn int    `json:"end_column,omitempty"`
}

func (c *check) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	// Use tool.EnsureAndRun to execute actionlint.
	// Falls back to the registry tool (cataloged with version prefix handling).
	cmd, err := tool.EnsureAndRunLazyWithFallbackAt(ctx, dir, "actionlint", "actionlint", "registry:actionlint", "-format", "{{json .}}")
	if err != nil {
		return nil, fmt.Errorf("setup actionlint: %w", err)
	}

	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// actionlint returns exit code 1 if issues are found, so ignore that when stdout has data.
	if err := cmd.Run(); err != nil && stdout.Len() == 0 {
		return nil, fmt.Errorf("actionlint execution failed: %w (stderr: %s)", err, stderr.String())
	}

	var issues []Issue
	if stdout.Len() > 0 {
		if err := json.Unmarshal(stdout.Bytes(), &issues); err != nil {
			return nil, fmt.Errorf("parse actionlint output: %w (stdout: %s)", err, stdout.String())
		}
	}

	if len(issues) == 0 {
		return nil, nil
	}

	// Build SARIF run from parsed issues, mirroring the official template structure.
	driver := sarif.NewDriver("actionlint")
	driver.InformationURI = checks.StringPtr(actionlintInfoURI)
	run := sarif.NewRun(*sarif.NewTool(driver))

	for _, issue := range issues {
		region := sarif.NewRegion().
			WithStartLine(issue.Line).
			WithStartColumn(issue.Column)

		if issue.EndColumn > 0 {
			region.WithEndColumn(issue.EndColumn)
		}

		loc := sarif.NewLocation().
			WithPhysicalLocation(sarif.NewPhysicalLocation().
				WithArtifactLocation(sarif.NewArtifactLocation().WithUri(issue.Filepath)).
				WithRegion(region))

		run.AddResult(
			sarif.NewRuleResult(issue.Kind).
				WithLevel("error").
				WithMessage(sarif.NewTextMessage(issue.Message)).
				WithLocations([]*sarif.Location{loc}),
		)
	}

	return run, nil
}
