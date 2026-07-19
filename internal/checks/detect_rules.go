package checks

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DetectResult is the outcome of firewall evaluation for one tool.
type DetectResult struct {
	Applicable bool
	// Winning rule key when a rule matched (even if enable=false).
	RuleKey string
	// Glob from the winning rule (for args_from_globs).
	Glob string
}

// EvaluateDetect applies ordered firewall rules. First matching rule wins.
// No match or empty detect → not applicable.
func EvaluateDetect(root string, rules map[string]DetectRule) (DetectResult, error) {
	if len(rules) == 0 {
		return DetectResult{}, nil
	}
	keys := make([]string, 0, len(rules))
	for k := range rules {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		rule := rules[key]
		matched, err := ruleMatches(root, rule)
		if err != nil {
			return DetectResult{}, fmt.Errorf("detect %q: %w", key, err)
		}
		if !matched {
			continue
		}
		return DetectResult{
			Applicable: rule.Enable,
			RuleKey:    key,
			Glob:       strings.TrimSpace(rule.Glob),
		}, nil
	}
	return DetectResult{}, nil
}

func ruleMatches(root string, rule DetectRule) (bool, error) {
	path := strings.TrimSpace(rule.Path)
	glob := strings.TrimSpace(rule.Glob)
	if path == "" && glob == "" {
		return false, nil
	}
	if path != "" {
		full := filepath.Join(root, filepath.FromSlash(path))
		if _, err := os.Stat(full); err != nil {
			if os.IsNotExist(err) {
				// path miss: if only path, no match; if also glob, try glob
				if glob == "" {
					return false, nil
				}
			} else {
				return false, err
			}
		} else {
			// path exists → match (AND with glob if both set)
			if glob == "" {
				return true, nil
			}
		}
	}
	if glob != "" {
		files, err := CollectGlob(root, glob)
		if err != nil {
			return false, err
		}
		if len(files) == 0 {
			return false, nil
		}
		// if both path and glob: path must exist AND glob hits
		if path != "" {
			full := filepath.Join(root, filepath.FromSlash(path))
			if _, err := os.Stat(full); err != nil {
				return false, nil
			}
		}
		return true, nil
	}
	return false, nil
}

// CollectGlob returns paths relative to root matching pattern.
// Supports ** and a single {a,b} brace group. Skips heavy dirs.
func CollectGlob(root, pattern string) ([]string, error) {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	if pattern == "" {
		return nil, nil
	}
	patterns := expandBraces(pattern)
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			switch name {
			case ".git", "node_modules", "vendor", ".workspaced", "dist", "build":
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		for _, p := range patterns {
			if matchGlob(p, relSlash) {
				out = append(out, rel)
				break
			}
		}
		return nil
	})
	return out, err
}

func expandBraces(pattern string) []string {
	start := strings.Index(pattern, "{")
	end := strings.Index(pattern, "}")
	if start < 0 || end < 0 || end < start {
		return []string{pattern}
	}
	prefix := pattern[:start]
	suffix := pattern[end+1:]
	inner := pattern[start+1 : end]
	parts := strings.Split(inner, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, prefix+strings.TrimSpace(p)+suffix)
	}
	return out
}

func matchGlob(pattern, name string) bool {
	pattern = filepath.ToSlash(pattern)
	name = filepath.ToSlash(name)
	// **/prefix
	if strings.HasPrefix(pattern, "**/") {
		rest := pattern[3:]
		// match rest against any suffix path segment chain
		if ok, _ := filepath.Match(rest, name); ok {
			return true
		}
		// any directory prefix
		for i := 0; i < len(name); i++ {
			if name[i] == '/' {
				if ok, _ := filepath.Match(rest, name[i+1:]); ok {
					return true
				}
			}
		}
		// also match rest as path.Match on basename-style **/*.ext
		return false
	}
	if strings.Contains(pattern, "**") {
		// limited: only leading **/ handled above
		return false
	}
	ok, _ := filepath.Match(pattern, name)
	return ok
}
