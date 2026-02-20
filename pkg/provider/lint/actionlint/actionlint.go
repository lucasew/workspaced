package actionlint

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"workspaced/pkg/provider/lint"
	"workspaced/pkg/tool"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// Provider implements the lint.Linter interface for GitHub Actions workflows.
type Provider struct{}

// New creates a new actionlint provider.
func New() lint.Linter {
	return &Provider{}
}

func init() {
	lint.Register(New())
}

func (p *Provider) Name() string {
	return "actionlint"
}

func (p *Provider) Detect(ctx context.Context, dir string) (bool, error) {
	// Applies if .github/workflows exists and is not empty
	workflowsDir := filepath.Join(dir, ".github", "workflows")
	entries, err := os.ReadDir(workflowsDir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(entries) == 0 {
		return false, nil
	}
	return true, nil
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

func (p *Provider) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	// Use tool.EnsureAndRun to execute actionlint.
	// This automatically handles installation and version resolution.
	cmd, err := tool.EnsureAndRun(ctx, "github:rhysd/actionlint@v1.7.11", "actionlint", "-format", "{{json .}}")
	if err != nil {
		return nil, fmt.Errorf("failed to setup actionlint: %w", err)
	}

	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// actionlint returns exit code 1 if issues are found, so we ignore exit code check errors if output is present
	if err := cmd.Run(); err != nil {
		// If stdout is empty, it's a real error (e.g. tool failure)
		if stdout.Len() == 0 {
			return nil, fmt.Errorf("actionlint execution failed: %w (stderr: %s)", err, stderr.String())
		}
		// Otherwise, issues were found, proceed to parsing
	}

	// Parse JSON output
	var issues []Issue
	if stdout.Len() > 0 {
		if err := json.Unmarshal(stdout.Bytes(), &issues); err != nil {
			return nil, fmt.Errorf("failed to parse actionlint output: %w (stdout: %s)", err, stdout.String())
		}
	}

	if len(issues) == 0 {
		return nil, nil
	}

	// Create SARIF run
	driver := sarif.NewDriver(p.Name())
	infoURI := "https://github.com/rhysd/actionlint"
	driver.InformationURI = &infoURI
	run := sarif.NewRun(*sarif.NewTool(driver))

	for _, issue := range issues {
		// Convert issue to SARIF result
		msg := sarif.NewTextMessage(issue.Message)

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

		res := sarif.NewRuleResult(issue.Kind).
			WithMessage(msg).
			WithLocations([]*sarif.Location{loc})

		run.AddResult(res)
	}

	return run, nil
}
