package resolution

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"github.com/lucasew/workspaced/internal/semver"
	"github.com/lucasew/workspaced/pkg/logging"
)

var ErrToolNotFound = errors.New("tool not found")

type Resolver struct {
	toolsDir string
}

func NewResolver(toolsDir string) *Resolver {
	return &Resolver{toolsDir: toolsDir}
}

func (r *Resolver) Resolve(ctx context.Context, toolName string) (string, error) {
	version, err := r.resolveVersion(ctx, toolName)
	if err != nil {
		return "", err
	}

	// Search for the tool binary in installed packages
	entries, err := os.ReadDir(r.toolsDir)
	if err != nil {
		return "", err
	}

	var candidates []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// entry.Name() is e.g. "github-denoland-deno"
		pkgDir := filepath.Join(r.toolsDir, entry.Name())

		// Check if the requested version exists for this package
		verDir := filepath.Join(pkgDir, version)

		// If version is "latest", we need to find the latest installed version for this package
		if version == "latest" {
			vers, err := os.ReadDir(pkgDir)
			if err != nil {
				continue
			}

			var versionNames []string
			for _, v := range vers {
				if v.IsDir() {
					versionNames = append(versionNames, v.Name())
				}
			}
			sortVersions(versionNames)

			// Iterate reversed to find latest
			for i := len(versionNames) - 1; i >= 0; i-- {
				vName := versionNames[i]
				binPath := r.checkBin(filepath.Join(pkgDir, vName), toolName)
				if binPath != "" {
					candidates = append(candidates, binPath)
					break // Found latest version for this package
				}
			}
		} else {
			// Check specific version
			if _, err := os.Stat(verDir); err == nil {
				binPath := r.checkBin(verDir, toolName)
				if binPath != "" {
					candidates = append(candidates, binPath)
				}
			}
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("%w: %q (version: %s)", ErrToolNotFound, toolName, version)
	}

	// If multiple candidates, pick one.
	// Maybe prefer exact matches on package name?
	// e.g. if toolName is "deno", prefer "github-denoland-deno"?
	// For now, just return first.
	return candidates[0], nil
}

func (r *Resolver) checkBin(verDir, toolName string) string {
	candidates := []string{
		filepath.Join(verDir, "bin", toolName),
		filepath.Join(verDir, "bin", toolName+".exe"),
		filepath.Join(verDir, toolName),
		filepath.Join(verDir, toolName+".exe"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func (r *Resolver) resolveVersion(ctx context.Context, toolName string) (string, error) {
	// 1. .tool-versions
	v, err := r.findInToolVersions(ctx, toolName)
	if err != nil {
		return "", err
	}
	if v != "" {
		return v, nil
	}

	// 2. Env var: WORKSPACED_TOOL_VERSION
	envKey := "WORKSPACED_" + strings.ToUpper(strings.ReplaceAll(toolName, "-", "_")) + "_VERSION"
	if v := os.Getenv(envKey); v != "" {
		return v, nil
	}

	// 3. Fallback
	return "latest", nil
}

func (r *Resolver) findInToolVersions(ctx context.Context, toolName string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", nil
	}

	for {
		p := filepath.Join(cwd, ".tool-versions")
		if _, err := os.Stat(p); err == nil {
			v, err := readToolVersion(ctx, p, toolName)
			if err != nil {
				return "", err
			}
			if v != "" {
				return v, nil
			}
		}

		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}

	home, err := os.UserHomeDir()
	if err == nil {
		p := filepath.Join(home, ".config", "workspaced", ".tool-versions")
		if _, err := os.Stat(p); err == nil {
			v, err := readToolVersion(ctx, p, toolName)
			if err != nil {
				return "", err
			}
			if v != "" {
				return v, nil
			}
		}
	}

	return "", nil
}

func readToolVersion(ctx context.Context, path, toolName string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer logging.Close(ctx, f)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[0] == toolName {
			return parts[1], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan %s: %w", path, err)
	}
	return "", nil
}

func sortVersions(versions []string) {
	sort.Slice(versions, func(i, j int) bool {
		return semver.Parse(versions[i]).Less(semver.Parse(versions[j]))
	})
}
