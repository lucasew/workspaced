package codec

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/lucasew/workspaced/internal/checks"
	"github.com/owenrumney/go-sarif/v2/sarif"
)

const actionlintInfoURI = "https://github.com/rhysd/actionlint"

type actionlintIssue struct {
	Message   string `json:"message"`
	Filepath  string `json:"filepath"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Kind      string `json:"kind"`
	EndColumn int    `json:"end_column,omitempty"`
}

func decodeActionlint(toolName string, data []byte) (*sarif.Run, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	var issues []actionlintIssue
	if err := json.Unmarshal(data, &issues); err != nil {
		return nil, fmt.Errorf("parse actionlint output: %w", err)
	}
	if len(issues) == 0 {
		return nil, nil
	}
	name := toolName
	if name == "" {
		name = "actionlint"
	}
	driver := sarif.NewDriver(name)
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
