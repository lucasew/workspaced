package codec

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/owenrumney/go-sarif/v2/sarif"
	"github.com/lucasew/workspaced/internal/checks"
)

const shellcheckInfoURI = "https://github.com/koalaman/shellcheck"

type shellcheckIssue struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	EndLine   int    `json:"endLine"`
	Column    int    `json:"column"`
	EndColumn int    `json:"endColumn"`
	Level     string `json:"level"`
	Code      int    `json:"code"`
	Message   string `json:"message"`
}

func decodeShellcheck(toolName string, data []byte) (*sarif.Run, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	var issues []shellcheckIssue
	if err := json.Unmarshal(data, &issues); err != nil {
		return nil, fmt.Errorf("parse shellcheck output: %w", err)
	}
	if len(issues) == 0 {
		return nil, nil
	}
	name := toolName
	if name == "" {
		name = "shellcheck"
	}
	driver := sarif.NewDriver(name)
	driver.InformationURI = checks.StringPtr(shellcheckInfoURI)
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
	return run, nil
}
