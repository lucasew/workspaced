package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/driver/notification"
)

var (
	// ErrArchiveNeedsInputAndOutput is returned when an archive action is missing input_dir or output.
	ErrArchiveNeedsInputAndOutput = errors.New("archive action requires input_dir and output")
	// ErrUnsupportedArchiveFormat is returned when an unsupported archive format is requested.
	ErrUnsupportedArchiveFormat = errors.New("unsupported archive format")
)

func init() {
	registerAction[ArchiveAction]("archive")
}

type ArchiveAction struct {
	backupActionBase
	InputDir string `json:"input_dir"`
	Output   string `json:"output"`
	Format   string `json:"format"`
}

func (action ArchiveAction) Run(ctx context.Context, _ *notification.Notification) error {
	if strings.TrimSpace(action.InputDir) == "" || strings.TrimSpace(action.Output) == "" {
		return ErrArchiveNeedsInputAndOutput
	}
	if action.Format != "tar" {
		return fmt.Errorf("%w: %s", ErrUnsupportedArchiveFormat, action.Format)
	}
	parent := filepath.Dir(action.InputDir)
	base := filepath.Base(action.InputDir)
	cmd := execdriver.MustRun(ctx, "tar", "-cvf", action.Output, "-C", parent, base)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
