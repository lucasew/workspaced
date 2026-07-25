package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/workspaced/internal/atomicfile"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/driver/notification"
	"github.com/lucasew/workspaced/pkg/logging"
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

	outDir := filepath.Dir(action.Output)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create archive output dir: %w", err)
	}

	tmp, err := os.CreateTemp(outDir, filepath.Base(action.Output)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create archive temp: %w", err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close archive temp: %w", err)
	}
	defer logging.RunCleanup(ctx, "remove", func() error {
		if err := os.Remove(tmpPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}, "path", tmpPath)

	parent := filepath.Dir(action.InputDir)
	base := filepath.Base(action.InputDir)
	cmd := execdriver.MustRun(ctx, "tar", "-cvf", tmpPath, "-C", parent, base)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	if err := atomicfile.Install(tmpPath, action.Output, 0); err != nil {
		return fmt.Errorf("install archive: %w", err)
	}
	return nil
}
