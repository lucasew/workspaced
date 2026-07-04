package checks

import (
	"errors"
	"os"
	"path/filepath"
)

// RequireFile returns ErrNotApplicable when name is missing under dir.
// Other stat errors are ignored so Detect stays permissive (same as prior
// inline missing-file checks).
func RequireFile(dir, name string) error {
	if _, err := os.Stat(filepath.Join(dir, name)); errors.Is(err, os.ErrNotExist) {
		return ErrNotApplicable
	}
	return nil
}
