package codec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

type eslintMessage struct {
	RuleID    string `json:"ruleId"`
	Severity  int    `json:"severity"`
	Message   string `json:"message"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	EndLine   int    `json:"endLine"`
	EndColumn int    `json:"endColumn"`
}

type eslintResult struct {
	FilePath string          `json:"filePath"`
	Messages []eslintMessage `json:"messages"`
}

const (
	eslintSeverityWarning = 1
	eslintSeverityError   = 2
)

func decodeESLint(data []byte) (*sarif.Run, error) {
	return parseAndConvertESLint(data)
}

func parseAndConvertESLint(output []byte) (*sarif.Run, error) {
	results, err := parseESLintResults(sanitizeESLintOutput(output))
	if err != nil {
		return nil, fmt.Errorf("unmarshal eslint output: %w", err)
	}
	run := sarif.NewRun(*sarif.NewTool(sarif.NewDriver("eslint")))
	for _, result := range results {
		for _, msg := range result.Messages {
			level := "warning"
			if msg.Severity == eslintSeverityError {
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

func sanitizeESLintOutput(raw []byte) []byte {
	b := bytes.TrimSpace(raw)
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	if len(b) == 0 {
		return b
	}
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

func parseESLintResults(output []byte) ([]eslintResult, error) {
	var results []eslintResult
	if err := json.Unmarshal(output, &results); err == nil {
		return results, nil
	}
	var encoded string
	if err := json.Unmarshal(output, &encoded); err == nil {
		var decoded []eslintResult
		if err := json.Unmarshal([]byte(encoded), &decoded); err == nil {
			return decoded, nil
		}
	}
	dec := json.NewDecoder(bytes.NewReader(output))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '[' {
		return nil, fmt.Errorf("unexpected top-level JSON token: %v", tok)
	}
	partial := make([]eslintResult, 0, 8)
	for dec.More() {
		var r eslintResult
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
