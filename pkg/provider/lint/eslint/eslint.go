package eslint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	os_exec "os/exec"
	"path/filepath"

	"workspaced/pkg/driver/exec"
	"workspaced/pkg/provider/lint"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// ErrBinaryNotFound is returned when neither node nor bun are found in PATH.
var ErrBinaryNotFound = errors.New("neither node nor bun found in PATH")

// Provider implements the lint.Linter interface for ESLint.
type Provider struct{}

// New creates a new ESLint provider.
func New() lint.Linter {
	return &Provider{}
}

func init() {
	lint.Register(New())
}

func (p *Provider) Name() string {
	return "eslint"
}

func (p *Provider) Detect(_ context.Context, dir string) (bool, error) {
	path := filepath.Join(dir, "node_modules", ".bin", "eslint")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}

type Message struct {
	RuleID    string `json:"ruleId"`
	Severity  int    `json:"severity"`
	Message   string `json:"message"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	NodeType  string `json:"nodeType"`
	MessageID string `json:"messageId"`
	EndLine   int    `json:"endLine"`
	EndColumn int    `json:"endColumn"`
}

type Result struct {
	FilePath string    `json:"filePath"`
	Messages []Message `json:"messages"`
}

const (
	SeverityWarning = 1
	SeverityError   = 2
)

func parseAndConvert(output []byte) (*sarif.Run, error) {
	var results []Result
	if err := json.Unmarshal(output, &results); err != nil {
		return nil, fmt.Errorf("failed to unmarshal eslint output: %w", err)
	}

	run := sarif.NewRun(*sarif.NewTool(sarif.NewDriver("eslint")))

	for _, result := range results {
		for _, msg := range result.Messages {
			level := "warning"
			if msg.Severity == SeverityError {
				level = "error"
			}

			sarifResult := sarif.NewRuleResult(msg.RuleID).
				WithMessage(sarif.NewTextMessage(msg.Message)).
				WithLevel(level).
				WithLocations([]*sarif.Location{
					sarif.NewLocation().
						WithPhysicalLocation(sarif.NewPhysicalLocation().
							WithArtifactLocation(sarif.NewArtifactLocation().
								WithUri(result.FilePath)).
							WithRegion(sarif.NewRegion().
								WithStartLine(msg.Line).
								WithStartColumn(msg.Column).
								WithEndLine(msg.EndLine).
								WithEndColumn(msg.EndColumn))),
				})
			run.AddResult(sarifResult)
		}
	}

	return run, nil
}

func (p *Provider) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	binPath := filepath.Join(dir, "node_modules", ".bin", "eslint")

	var cmd *os_exec.Cmd
	var err error

	if exec.IsBinaryAvailable(ctx, "node") {
		cmd, err = exec.Run(ctx, binPath, "-f", "json", ".")
	} else if exec.IsBinaryAvailable(ctx, "bun") {
		cmd, err = exec.Run(ctx, "bun", "run", "--bun", binPath, "-f", "json", ".")
	} else {
		return nil, ErrBinaryNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to prepare eslint command: %w", err)
	}

	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		var exitErr *os_exec.ExitError
		// ESLint returns exit code 1 if errors found, which is fine if output is valid JSON.
		if !errors.As(err, &exitErr) {
			// Not an exit error (e.g. not found), return error.
			return nil, fmt.Errorf("eslint execution failed: %w", err)
		}
	}

	run, err := parseAndConvert(stdout.Bytes())
	if err != nil {
		slog.Error("eslint failed to produce valid JSON", "stderr", stderr.String())
		return nil, fmt.Errorf("eslint failed: %w: %s", err, stderr.String())
	}

	return run, nil
}
