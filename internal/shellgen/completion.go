package shellgen

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrRootCommandNotSet is returned when shell completion is requested before setting the root command.
	ErrRootCommandNotSet = errors.New("root command not set, call SetRootCommand first")
)

// GenerateCompletion generates bash completion using cobra API directly (no exec)
func GenerateCompletion() (string, error) {
	if rootCommand == nil {
		return "", ErrRootCommandNotSet
	}

	// Generate completion directly using cobra API (much faster than exec)
	var buf strings.Builder
	if err := rootCommand.GenBashCompletionV2(&buf, true); err != nil {
		return "", fmt.Errorf("generate bash completion: %w", err)
	}

	return buf.String(), nil
}
