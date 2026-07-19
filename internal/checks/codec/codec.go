// Package codec converts linter stdout into SARIF runs.
package codec

import (
	"fmt"

	"github.com/owenrumney/go-sarif/v2/sarif"
	"github.com/lucasew/workspaced/internal/checks"
)

// Name identifies a closed set of stdout codecs.
type Name string

const (
	SARIF          Name = "sarif"
	ActionlintJSON Name = "actionlint_json"
	ShellcheckJSON Name = "shellcheck_json"
	ESLintJSON     Name = "eslint_json"
)

// Decode converts tool stdout into a SARIF run (nil run = no findings).
func Decode(name string, toolName string, data []byte) (*sarif.Run, error) {
	switch Name(name) {
	case SARIF, "":
		return checks.FirstSARIFRun(data)
	case ActionlintJSON:
		return decodeActionlint(toolName, data)
	case ShellcheckJSON:
		return decodeShellcheck(toolName, data)
	case ESLintJSON:
		return decodeESLint(data)
	default:
		return nil, fmt.Errorf("unknown lint output codec %q", name)
	}
}
