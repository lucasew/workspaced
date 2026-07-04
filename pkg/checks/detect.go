package checks

import (
	"os"
	"path/filepath"
)

// RequireFile returns ErrNotApplicable when name is missing under dir.
// Other stat errors are ignored so Detect stays permissive (same as prior
// inline os.IsNotExist checks).
func RequireFile(dir, name string) error {
	if _, err := os.Stat(filepath.Join(dir, name)); os.IsNotExist(err) {
		return ErrNotApplicable
	}
	return nil
}
