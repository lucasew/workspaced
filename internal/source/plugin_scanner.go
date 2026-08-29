package source

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrScannerNameRequired is returned when a scanner plugin is created without a name.
	ErrScannerNameRequired = errors.New("scanner name is required")
	// ErrBaseDirectoryRequired is returned when a scanner plugin is created without a base directory.
	ErrBaseDirectoryRequired = errors.New("base directory is required")
	// ErrBaseDirNotExist is returned when the scanner's base directory does not exist.
	ErrBaseDirNotExist = errors.New("base directory does not exist")
)

// ScannerPlugin discovers files in a directory.
type ScannerPlugin struct {
	name       string
	baseDir    string
	targetBase string
	priority   int
}

// ScannerConfig configures a ScannerPlugin.
type ScannerConfig struct {
	Name       string // Unique identifier
	BaseDir    string // Source directory (where the files are)
	TargetBase string // Base path for targets (optional, default: $HOME)
	Priority   int    // Priority in conflicts
}

// NewScannerPlugin creates a scanner plugin.
func NewScannerPlugin(cfg ScannerConfig) (*ScannerPlugin, error) {
	if cfg.Name == "" {
		return nil, ErrScannerNameRequired
	}
	if cfg.BaseDir == "" {
		return nil, ErrBaseDirectoryRequired
	}

	// Expand paths
	baseDir := os.ExpandEnv(cfg.BaseDir)
	if strings.HasPrefix(baseDir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		baseDir = filepath.Join(home, baseDir[2:])
	}

	if _, err := os.Stat(baseDir); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrBaseDirNotExist, baseDir)
	}

	// Target base (default: $HOME)
	targetBase := cfg.TargetBase
	if targetBase == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		targetBase = home
	}

	return &ScannerPlugin{
		name:       cfg.Name,
		baseDir:    baseDir,
		targetBase: targetBase,
		priority:   cfg.Priority,
	}, nil
}

func (p *ScannerPlugin) Name() string {
	return fmt.Sprintf("scanner:%s", p.name)
}

func (p *ScannerPlugin) Process(ctx context.Context, files []File) ([]File, error) {
	discovered := []File{}

	err := filepath.Walk(p.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(p.baseDir, path)
		if err != nil {
			return err
		}

		fileType := TypeStatic
		if info.Mode()&os.ModeSymlink != 0 {
			fileType = TypeSymlink
		}

		discovered = append(discovered, &StaticFile{
			BasicFile: BasicFile{
				RelPathStr:    rel,
				TargetBaseDir: p.targetBase,
				FileMode:      info.Mode(),
				Info:          fmt.Sprintf("source:%s (%s)", p.name, rel),
				FileType:      fileType,
			},
			AbsPath: path,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	return append(files, discovered...), nil
}
