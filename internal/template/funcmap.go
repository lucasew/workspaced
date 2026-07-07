package template

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	envdriver "workspaced/pkg/driver/env"
	shimdriver "workspaced/pkg/driver/shim"
	"workspaced/internal/icons"
	"workspaced/pkg/logging"
	"workspaced/internal/modfile"
	"workspaced/internal/text"
)

// ErrFileSkipped is returned when a template calls {{ skip }}.
var ErrFileSkipped = errors.New("file skipped")

// makeFuncMap creates the default workspaced FuncMap.
func makeFuncMap(ctx context.Context) template.FuncMap {
	lockTool, lockSource := makeLockLookups(ctx)
	return template.FuncMap{
		"skip": func() (string, error) {
			return "", ErrFileSkipped
		},
		"dotfiles": func() (string, error) {
			return envdriver.GetDotfilesRoot(ctx)
		},
		"home": func() (string, error) {
			return envdriver.GetHomeDir(ctx)
		},
		"userDataDir": func() (string, error) {
			return envdriver.GetUserDataDir(ctx)
		},
		"file": func(name string, mode ...string) string {
			perm := "0644"
			if len(mode) > 0 {
				perm = mode[0]
			}
			return fmt.Sprintf("%s%s:%s>>>\n", markerFileStart, name, perm)
		},
		"endfile": func() string {
			return fmt.Sprintf("\n%s\n", markerFileEnd)
		},
		// String functions
		"split": func(s, sep string) []string {
			return strings.Split(s, sep)
		},
		"join": func(arr []string, sep string) string {
			return strings.Join(arr, sep)
		},
		"trimSpace": strings.TrimSpace,
		"replace": func(s, old, new string) string {
			return strings.ReplaceAll(s, old, new)
		},
		// Array/slice functions
		"list": func(items ...any) []any {
			return items
		},
		"last": func(arr []string) string {
			if len(arr) == 0 {
				return ""
			}
			return arr[len(arr)-1]
		},
		// Logic helpers
		"default": func(def any, val any) any {
			if val == nil || val == "" {
				return def
			}
			return val
		},
		"ternary": func(condition bool, trueVal, falseVal any) any {
			if condition {
				return trueVal
			}
			return falseVal
		},
		// Webapp helpers
		"favicon": func(url string) (string, error) {
			return getFavicon(ctx, url)
		},
		"isWayland": func() bool {
			return os.Getenv("WAYLAND_DISPLAY") != ""
		},
		"titleCase": func(s string) string {
			return text.ToTitleCase(s)
		},
		"normalizeURL": func(url string) string {
			return envdriver.NormalizeURL(url)
		},
		// Filesystem helpers
		"readDir": func(path string) ([]string, error) {
			entries, err := os.ReadDir(path)
			if err != nil {
				return nil, err
			}
			var names []string
			for _, entry := range entries {
				if !entry.IsDir() {
					names = append(names, entry.Name())
				}
			}
			return names, nil
		},
		"isPhone": func() bool {
			return envdriver.IsPhone(ctx)
		},
		// Shim helpers
		"shim": func(command ...string) (string, error) {
			return shimdriver.GenerateContent(ctx, command)
		},
		"lockTool":   lockTool,
		"lockSource": lockSource,
	}
}

func makeLockLookups(ctx context.Context) (func(string) map[string]any, func(string) map[string]any) {
	var once sync.Once
	var sum *modfile.SumFile
	load := func() *modfile.SumFile {
		once.Do(func() {
			dotfilesRoot, err := envdriver.GetDotfilesRoot(ctx)
			if err != nil {
				sum = &modfile.SumFile{}
				return
			}
			loaded, err := modfile.LoadSumFile(filepath.Join(dotfilesRoot, "workspaced.lock.json"))
			if err != nil {
				sum = &modfile.SumFile{}
				return
			}
			sum = loaded
		})
		return sum
	}

	lockTool := func(name string) map[string]any {
		locked, ok := load().Tool(name)
		if !ok {
			return nil
		}
		return map[string]any{
			"ref":     locked.Ref,
			"version": locked.Version,
		}
	}

	lockSource := func(name string) map[string]any {
		locked, ok := load().Source(name)
		if !ok {
			return nil
		}
		return map[string]any{
			"provider": locked.Provider,
			"path":     locked.Path,
			"repo":     locked.Repo,
			"ref":      locked.Ref,
			"url":      locked.URL,
			"hash":     locked.Hash,
		}
	}

	return lockTool, lockSource
}

func getFavicon(ctx context.Context, url string) (string, error) {
	iconPath, err := icons.GetIconPath(ctx, url)
	if err != nil {
		logger := logging.GetLogger(ctx)
		logger.Error("failed to get favicon", "url", url, "error", err)
		// Return fallback icon
		return "applications-internet", nil
	}
	return iconPath, nil
}
