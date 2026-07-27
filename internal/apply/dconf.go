package apply

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/lucasew/workspaced/internal/configcue"
	"github.com/lucasew/workspaced/internal/source"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/logging"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DconfPlugin emits a content-hash marker so dconf changes participate in deploy planning.
// Implements source.Plugin directly (no legacy Provider adapter).
type DconfPlugin struct{}

func (p *DconfPlugin) Name() string {
	return "dconf"
}

func (p *DconfPlugin) Process(ctx context.Context, files []source.File) ([]source.File, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dconfContent, err := buildHomeDconfContent(ctx)
	if err != nil {
		return nil, err
	}
	if dconfContent == "" {
		return files, nil
	}

	// Use content hash as marker to track changes
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(dconfContent)))

	marker := &source.BufferFile{
		BasicFile: source.BasicFile{
			RelPathStr:    "dconf.marker",
			TargetBaseDir: filepath.Join(home, ".config", "workspaced"),
			FileMode:      0644,
			Info:          "dconf (marker)",
			FileType:      source.TypeStatic,
		},
		Content: []byte(hash),
	}
	return append(files, marker), nil
}

func ApplyHomeDconf(ctx context.Context) error {
	dconfContent, err := buildHomeDconfContent(ctx)
	if err != nil {
		return err
	}
	if dconfContent == "" {
		return nil
	}

	// Unique temp path per call so concurrent home apply / overlapping runs
	// cannot clobber each other's ini mid-`dconf load` (fixed name was a race).
	tmpIni, err := writeTempDconfIni(ctx, dconfContent)
	if err != nil {
		return err
	}
	defer logging.RunCleanup(ctx, "remove", func() error { return os.Remove(tmpIni) })
	return applyDconf(ctx, tmpIni)
}

// writeTempDconfIni writes content to a unique temp file (0600). Caller removes it.
func writeTempDconfIni(ctx context.Context, content string) (string, error) {
	f, err := os.CreateTemp("", "workspaced-dconf-*.ini")
	if err != nil {
		return "", err
	}
	path := f.Name()
	if _, err := f.WriteString(content); err != nil {
		logging.Close(ctx, f)
		if rmErr := os.Remove(path); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			logging.ReportError(ctx, rmErr, "path", path)
		}
		return "", err
	}
	if err := f.Close(); err != nil {
		if rmErr := os.Remove(path); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			logging.ReportError(ctx, rmErr, "path", path)
		}
		return "", err
	}
	return path, nil
}

func buildHomeDconfContent(ctx context.Context) (string, error) {
	cfg, err := configcue.LoadHome(ctx)
	if err != nil {
		return "", err
	}

	rawDconf := make(map[string]map[string]any)
	if err := cfg.Decode("desktop.raw.dconf", &rawDconf); err != nil {
		rawDconf = make(map[string]map[string]any)
	}

	var darkMode bool
	if err := cfg.Decode("desktop.dark_mode", &darkMode); err == nil {
		if rawDconf["org/gnome/desktop/interface"] == nil {
			rawDconf["org/gnome/desktop/interface"] = make(map[string]any)
		}
		if darkMode {
			rawDconf["org/gnome/desktop/interface"]["color-scheme"] = "prefer-dark"
		} else {
			rawDconf["org/gnome/desktop/interface"]["color-scheme"] = "prefer-light"
		}
	}

	if len(rawDconf) == 0 {
		return "", nil
	}

	var sb strings.Builder
	paths := make([]string, 0, len(rawDconf))
	for path := range rawDconf {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		settings := rawDconf[path]
		fmt.Fprintf(&sb, "[%s]\n", path)

		keys := make([]string, 0, len(settings))
		for key := range settings {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			value := settings[key]
			fmt.Fprintf(&sb, "%s=%s\n", key, formatDconfValue(value))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func applyDconf(ctx context.Context, iniFile string) error {
	cmd, err := execdriver.Run(ctx, "dconf", "load", "/")
	if err != nil {
		return err
	}

	file, err := os.Open(iniFile)
	if err != nil {
		return err
	}
	defer logging.Close(ctx, file)

	cmd.Stdin = file
	return cmd.Run()
}

func formatDconfValue(v any) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("'%s'", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case int, int64, float64:
		return fmt.Sprintf("%v", val)
	case []any:
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = formatDconfValue(item)
		}
		return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
	default:
		return fmt.Sprintf("%v", val)
	}
}
