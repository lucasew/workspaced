package icons

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"workspaced/pkg/env"
)

type ThemeGenerateOptions struct {
	InputDir       string
	OutputDir      string
	ThemeName      string
	Jobs           string
	Sizes          string
	Replace        []string
	MapScheme      bool
	HasMapScheme   bool
	Clean          bool
	NoRaster       bool
	UpdateCache    bool
	HasUpdateCache bool
	DefaultContext string
	UseCache       bool
	Stdout         io.Writer
	Stderr         io.Writer
}

func (o ThemeGenerateOptions) WithDefaults() ThemeGenerateOptions {
	if o.InputDir == "" {
		o.InputDir = "/tmp/papirus-icon-theme-20250501/Papirus"
	}
	if o.OutputDir == "" {
		o.OutputDir = "~/.local/share/icons/papirus-base16"
	}
	if o.ThemeName == "" {
		o.ThemeName = "papirus-base16"
	}
	if o.Jobs == "" {
		o.Jobs = "auto"
	}
	if o.Sizes == "" {
		o.Sizes = "16,24,32,48,64,128,256"
	}
	if o.DefaultContext == "" {
		o.DefaultContext = "apps"
	}
	if !o.HasMapScheme {
		o.MapScheme = true
	}
	if !o.HasUpdateCache {
		o.UpdateCache = true
	}
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	return o
}

func RunThemeGenerate(ctx context.Context, opts ThemeGenerateOptions) error {
	opts = opts.WithDefaults()
	inputDir := env.ExpandPath(opts.InputDir)
	outputDir := env.ExpandPath(opts.OutputDir)

	if _, err := os.Stat(inputDir); err != nil {
		return fmt.Errorf("icon source dir not found: %s", inputDir)
	}

	if opts.UseCache {
		sig, err := iconSourceSignature(inputDir, opts.ThemeName, opts.Jobs)
		if err != nil {
			return err
		}
		statePath := filepath.Join(outputDir, ".workspaced-icons-state")
		indexPath := filepath.Join(outputDir, "index.theme")
		if oldSig, err := os.ReadFile(statePath); err == nil {
			if strings.TrimSpace(string(oldSig)) == sig {
				if _, err := os.Stat(indexPath); err == nil {
					return nil
				}
			}
		}
		if err := runThemeGenerateEngine(ctx, opts, inputDir, outputDir); err != nil {
			return err
		}
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return err
		}
		return os.WriteFile(statePath, []byte(sig+"\n"), 0644)
	}

	return runThemeGenerateEngine(ctx, opts, inputDir, outputDir)
}

func iconSourceSignature(inputDir, themeName, jobs string) (string, error) {
	var count int64
	var sizeSum int64
	var maxMtime int64

	err := filepath.WalkDir(inputDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".svg") && !strings.HasSuffix(name, ".svg.tmpl") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		count++
		sizeSum += info.Size()
		mt := info.ModTime().Unix()
		if mt > maxMtime {
			maxMtime = mt
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	return strings.Join([]string{
		"v1",
		filepath.Clean(inputDir),
		themeName,
		jobs,
		strconv.FormatInt(count, 10),
		strconv.FormatInt(sizeSum, 10),
		strconv.FormatInt(maxMtime, 10),
	}, "|"), nil
}
