package shellcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"workspaced/pkg/checks"
	"workspaced/pkg/checks/lint"
	"workspaced/pkg/tool"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// check implements the lint.Linter interface for shell scripts.
// It executes shellcheck via the workspaced tool system (registry:shellcheck /
// koalaman/shellcheck releases).
type check struct{}

const shellcheckInfoURI = "https://github.com/koalaman/shellcheck"

// New creates a new ShellCheck check.
func New() lint.Linter {
	return &check{}
}

func init() {
	lint.Register(New())
}

func (c *check) Name() string {
	return "shellcheck"
}

func (c *check) Detect(_ context.Context, dir string) error {
	files, err := collectShellFiles(dir)
	if err != nil || len(files) == 0 {
		return checks.ErrNotApplicable
	}
	return nil
}

// Issue is the JSON object emitted by `shellcheck -f json`.
type Issue struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	EndLine   int    `json:"endLine"`
	Column    int    `json:"column"`
	EndColumn int    `json:"endColumn"`
	Level     string `json:"level"`
	Code      int    `json:"code"`
	Message   string `json:"message"`
}

func (c *check) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	files, err := collectShellFiles(dir)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	args := append([]string{"-f", "json"}, files...)
	// Falls back to registry:shellcheck (catalog entry normalizes v-prefixed tags).
	cmd, err := tool.EnsureAndRunLazyWithFallbackAt(ctx, dir, "shellcheck", "shellcheck", "registry:shellcheck", args...)
	if err != nil {
		return nil, fmt.Errorf("failed to setup shellcheck: %w", err)
	}

	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// shellcheck exits non-zero when issues are found; only fail when stdout is empty.
	if err := cmd.Run(); err != nil && stdout.Len() == 0 {
		return nil, fmt.Errorf("shellcheck execution failed: %w (stderr: %s)", err, stderr.String())
	}

	if stdout.Len() == 0 {
		return nil, nil
	}

	var issues []Issue
	if err := json.Unmarshal(stdout.Bytes(), &issues); err != nil {
		return nil, fmt.Errorf("failed to parse shellcheck output: %w (stdout: %s)", err, stdout.String())
	}
	if len(issues) == 0 {
		return nil, nil
	}

	return convertToSarif(issues), nil
}

func convertToSarif(issues []Issue) *sarif.Run {
	driver := sarif.NewDriver("shellcheck")
	driver.InformationURI = strPtr(shellcheckInfoURI)
	run := sarif.NewRun(*sarif.NewTool(driver))

	for _, issue := range issues {
		level := "warning"
		switch issue.Level {
		case "error":
			level = "error"
		case "info", "style":
			level = "note"
		}

		region := sarif.NewRegion().
			WithStartLine(issue.Line).
			WithStartColumn(issue.Column)
		if issue.EndLine > 0 {
			region.WithEndLine(issue.EndLine)
		}
		if issue.EndColumn > 0 {
			region.WithEndColumn(issue.EndColumn)
		}

		loc := sarif.NewLocation().
			WithPhysicalLocation(sarif.NewPhysicalLocation().
				WithArtifactLocation(sarif.NewArtifactLocation().WithUri(issue.File)).
				WithRegion(region))

		run.AddResult(
			sarif.NewRuleResult(fmt.Sprintf("SC%d", issue.Code)).
				WithLevel(level).
				WithMessage(sarif.NewTextMessage(issue.Message)).
				WithLocations([]*sarif.Location{loc}),
		)
	}
	return run
}

// collectShellFiles walks dir and returns paths (relative to dir) of shell scripts.
// Currently .sh files; skips heavy/non-source trees.
func collectShellFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			switch name {
			case ".git", "node_modules", "vendor", ".workspaced", "dist", "build":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".sh") {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	return files, err
}

func strPtr(s string) *string {
	return &s
}
