package eslint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	os_exec "os/exec"
	"path/filepath"
	"strings"

	"workspaced/pkg/driver/exec"
	"workspaced/pkg/provider"
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

func (p *Provider) Detect(_ context.Context, dir string) error {
	path := filepath.Join(dir, "node_modules", ".bin", "eslint")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return provider.ErrNotApplicable
	}

	return nil
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

func sanitizeESLintOutput(raw []byte) []byte {
	b := bytes.TrimSpace(raw)
	// Remove UTF-8 BOM if present.
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	if len(b) == 0 {
		return b
	}

	// If stdout contains extra text, extract the first JSON value.
	if b[0] != '[' && b[0] != '{' {
		if extracted, ok := extractFirstJSONValue(b); ok {
			b = extracted
		}
	}

	return escapeInvalidStringControlChars(b)
}

func extractFirstJSONValue(raw []byte) ([]byte, bool) {
	start := -1
	var opener byte
	for i, c := range raw {
		if c == '[' || c == '{' {
			start = i
			opener = c
			break
		}
	}
	if start == -1 {
		return nil, false
	}

	closer := byte(']')
	if opener == '{' {
		closer = '}'
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(raw); i++ {
		c := raw[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		switch c {
		case '"':
			inString = true
		case opener:
			depth++
		case closer:
			depth--
			if depth == 0 {
				return raw[start : i+1], true
			}
		}
	}

	return nil, false
}

func escapeInvalidStringControlChars(raw []byte) []byte {
	out := make([]byte, 0, len(raw))
	inString := false
	escaped := false

	for _, c := range raw {
		if inString {
			if escaped {
				out = append(out, c)
				escaped = false
				continue
			}

			switch c {
			case '\\':
				out = append(out, c)
				escaped = true
			case '"':
				out = append(out, c)
				inString = false
			case '\n':
				out = append(out, '\\', 'n')
			case '\r':
				out = append(out, '\\', 'r')
			case '\t':
				out = append(out, '\\', 't')
			default:
				if c < 0x20 {
					// Keep JSON valid if any other raw control character appears.
					out = append(out, ' ')
					continue
				}
				out = append(out, c)
			}
			continue
		}

		out = append(out, c)
		if c == '"' {
			inString = true
		}
	}

	return out
}

func parseAndConvert(output []byte) (*sarif.Run, error) {
	results, err := parseResults(sanitizeESLintOutput(output))
	if err != nil {
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

func parseResults(output []byte) ([]Result, error) {
	var results []Result
	if err := json.Unmarshal(output, &results); err == nil {
		return results, nil
	}

	// Some wrappers can double-encode output as a JSON string.
	var encoded string
	if err := json.Unmarshal(output, &encoded); err == nil {
		var decoded []Result
		if err := json.Unmarshal([]byte(encoded), &decoded); err == nil {
			return decoded, nil
		}
	}

	// Best-effort parse: if top-level array is truncated, keep decoded items.
	dec := json.NewDecoder(bytes.NewReader(output))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}

	delim, ok := tok.(json.Delim)
	if !ok || delim != '[' {
		return nil, fmt.Errorf("unexpected top-level JSON token: %v", tok)
	}

	partial := make([]Result, 0, 8)
	for dec.More() {
		var r Result
		if err := dec.Decode(&r); err != nil {
			if isTruncationError(err) && len(partial) > 0 {
				return partial, nil
			}
			return nil, err
		}
		partial = append(partial, r)
	}

	if _, err := dec.Token(); err != nil {
		if isTruncationError(err) && len(partial) > 0 {
			return partial, nil
		}
		return nil, err
	}

	return partial, nil
}

func isTruncationError(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "unexpected EOF") ||
		strings.Contains(msg, "unexpected end of JSON input")
}

func (p *Provider) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	binPath := filepath.Join(dir, "node_modules", ".bin", "eslint")

	var cmd *os_exec.Cmd
	var err error

	switch {
	case exec.IsBinaryAvailable(ctx, "node"):
		cmd, err = exec.Run(ctx, binPath, "-f", "json", ".")
	case exec.IsBinaryAvailable(ctx, "bun"):
		cmd, err = exec.Run(ctx, "bun", "run", "--bun", binPath, "-f", "json", ".")
	default:
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
		rawStdout := stdout.String()
		if len(rawStdout) > 2048 {
			rawStdout = rawStdout[:2048] + "...(truncated)"
		}
		slog.Error("eslint failed to produce valid JSON", "stderr", stderr.String(), "stdout", rawStdout)
		return nil, fmt.Errorf("eslint failed: %w: %s", err, stderr.String())
	}

	return run, nil
}
