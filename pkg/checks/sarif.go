package checks

import (
	"fmt"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// FirstSARIFRun parses SARIF bytes and returns the first run, or (nil, nil)
// when the report has no runs.
func FirstSARIFRun(data []byte) (*sarif.Run, error) {
	report, err := sarif.FromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("parse sarif output: %w", err)
	}
	if len(report.Runs) == 0 {
		return nil, nil
	}
	return report.Runs[0], nil
}

// StringPtr returns a pointer to s (for SARIF optional string fields).
func StringPtr(s string) *string {
	return &s
}
