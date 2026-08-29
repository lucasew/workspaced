package checks

import (
	"os"
	"os/exec"
)

// RunAttached runs cmd in dir with stdout/stderr attached to the process streams.
// Formatters use this so tool output stays visible to the user.
func RunAttached(cmd *exec.Cmd, dir string) error {
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
