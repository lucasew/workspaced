package resolution

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Resolver identifies the specific binary path for a tool based on version constraints.
// It searches installed packages in the tools directory and resolves versions from
// .tool-versions files, environment variables, or defaults to "latest".
type Resolver struct {
	toolsDir string
}

// NewResolver creates a new Resolver instance with the given tools directory.
func NewResolver(toolsDir string) *Resolver {
	return &Resolver{toolsDir: toolsDir}
}

// Resolve locates the executable binary for the specified tool name.
// It first determines the desired version, then scans installed packages for a match.
// If the version is "latest", it finds the most recent installed version.
// Returns the absolute path to the binary or an error if not found.
func (r *Resolver) Resolve(ctx context.Context, toolName string) (string, error) {
	version := r.resolveVersion(toolName)

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
		return "", fmt.Errorf("tool %q not found (version: %s)", toolName, version)
	}

	// If multiple candidates, pick one.
	// Maybe prefer exact matches on package name?
	// e.g. if toolName is "deno", prefer "github-denoland-deno"?
	// For now, just return first.
	return candidates[0], nil
}

func (r *Resolver) checkBin(verDir, toolName string) string {
	// Check bin/toolName
	p := filepath.Join(verDir, "bin", toolName)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	// Check bin/toolName.exe
	p = filepath.Join(verDir, "bin", toolName+".exe")
	if _, err := os.Stat(p); err == nil {
		return p
	}

	// Check toolName in root
	p = filepath.Join(verDir, toolName)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	// Check toolName.exe in root
	p = filepath.Join(verDir, toolName+".exe")
	if _, err := os.Stat(p); err == nil {
		return p
	}

	return ""
}

func (r *Resolver) resolveVersion(toolName string) string {
	// 1. .tool-versions
	if v := r.findInToolVersions(toolName); v != "" {
		return v
	}

	// 2. Env var: WORKSPACED_TOOL_VERSION
	envKey := "WORKSPACED_" + strings.ToUpper(strings.ReplaceAll(toolName, "-", "_")) + "_VERSION"
	if v := os.Getenv(envKey); v != "" {
		return v
	}

	// 3. Fallback
	return "latest"
}

func (r *Resolver) findInToolVersions(toolName string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		p := filepath.Join(cwd, ".tool-versions")
		if _, err := os.Stat(p); err == nil {
			if v := readToolVersion(p, toolName); v != "" {
				return v
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
			if v := readToolVersion(p, toolName); v != "" {
				return v
			}
		}
	}

	return ""
}

func readToolVersion(path, toolName string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() {
		// We can't do much if closing the file fails in this context,
		// and we are just reading configuration.
		_ = f.Close()
	}()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[0] == toolName {
			return parts[1]
		}
	}
	return ""
}

func sortVersions(versions []string) {
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j]) < 0
	})
}

func compareVersions(v1, v2 string) int {
	p1 := parseVersion(v1)
	p2 := parseVersion(v2)
	l := min(len(p2), len(p1))
	for k := 0; k < l; k++ {
		if p1[k] < p2[k] {
			return -1
		}
		if p1[k] > p2[k] {
			return 1
		}
	}
	if len(p1) < len(p2) {
		return -1
	}
	if len(p1) > len(p2) {
		return 1
	}
	return 0
}

func parseVersion(v string) []int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	var nums []int
	for _, part := range parts {
		numPart := ""
		for _, r := range part {
			if r >= '0' && r <= '9' {
				numPart += string(r)
			} else {
				break
			}
		}
		if numPart == "" {
			nums = append(nums, 0)
		} else {
			n, _ := strconv.Atoi(numPart)
			nums = append(nums, n)
		}
	}
	return nums
}
