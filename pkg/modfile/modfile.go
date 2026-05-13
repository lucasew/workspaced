package modfile

import (
	"fmt"
	"path/filepath"
	"strings"
	"workspaced/pkg/configcue"
	parsespec "workspaced/pkg/parse/spec"
)

type SourceConfig struct {
	Provider string `json:"provider"`
	Path     string `json:"path"`
	Repo     string `json:"repo"`
	URL      string `json:"url"`
	Ref      string `json:"ref"`
}

type ModFile struct {
	Sources map[string]SourceConfig `json:"sources"`
}

type ResolvedModuleSource struct {
	Provider string
	Ref      string
	Version  string
}

func LoadModFile(path string) (*ModFile, error) {
	_ = path
	return &ModFile{Sources: map[string]SourceConfig{}}, nil
}

func ModFileFromConfig(cfg *configcue.Config) (*ModFile, error) {
	out := &ModFile{Sources: map[string]SourceConfig{}}
	if cfg == nil {
		return out, nil
	}
	inputs, err := cfg.Inputs()
	if err != nil {
		return out, nil
	}
	for name, input := range inputs {
		spec := strings.TrimSpace(input.From)
		if spec == "" {
			continue
		}
		if spec == "self" {
			out.Sources[name] = SourceConfig{
				Provider: "self",
				Ref:      strings.TrimSpace(input.Version),
			}
			continue
		}
		src, err := ParseSourceSpec(spec)
		if err != nil {
			return nil, fmt.Errorf("invalid input %q from %q: %w", name, spec, err)
		}
		src.Ref = strings.TrimSpace(input.Version)
		out.Sources[name] = src
	}
	return out, nil
}

func (m *ModFile) ResolveModuleSource(moduleName, explicitFrom, modulesBaseDir string, sumFile *SumFile) (ResolvedModuleSource, error) {
	spec := strings.TrimSpace(explicitFrom)
	if spec == "" {
		spec = "self:modules/" + moduleName
	}

	parts := strings.SplitN(spec, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return ResolvedModuleSource{}, fmt.Errorf("invalid module source %q (expected <source-or-provider>:<path>[@version])", spec)
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])

	// Built-in providers (no alias entry required).
	if left == "self" || left == "core" || left == "github" || left == "registry" || left == "http" || left == "https" {
		ref, version := splitRefAndVersion(right)
		resolved, err := applyVersionLock(moduleName, left, ref, version, sumFile)
		if err != nil {
			return ResolvedModuleSource{}, err
		}
		return resolved, validateNonVersionedProvider(resolved)
	}

	// Alias source from workspaced inputs.
	src, ok := m.Sources[left]
	if !ok {
		return ResolvedModuleSource{}, fmt.Errorf("unknown source alias %q for module %q", left, moduleName)
	}
	src = applySourceLockOverlay(left, src, sumFile)
	if err := validateSourceLock(left, src, sumFile); err != nil {
		return ResolvedModuleSource{}, err
	}
	provider := strings.TrimSpace(src.Provider)
	if provider == "" {
		provider = left
	}

	switch provider {
	case "self":
		base := filepath.Dir(modulesBaseDir)
		customBase := strings.TrimSpace(src.Path)
		if customBase != "" {
			if filepath.IsAbs(customBase) {
				base = customBase
			} else {
				base = filepath.Join(base, customBase)
			}
		}
		ref, version := splitRefAndVersion(right)
		resolved, err := applyVersionLock(moduleName, "self", filepath.Join(base, ref), version, sumFile)
		if err != nil {
			return ResolvedModuleSource{}, err
		}
		return resolved, validateNonVersionedProvider(resolved)
	case "github":
		repo := normalizeGitHubRepo(src.Repo)
		if repo == "" {
			return ResolvedModuleSource{}, fmt.Errorf("source alias %q (github) requires repo", left)
		}
		ref, version := splitRefAndVersion(right)
		if version == "" {
			version = strings.TrimSpace(src.Ref)
		}
		path := strings.Trim(strings.TrimSpace(ref), "/")
		fullRef := repo
		if path != "" {
			fullRef = repo + "/" + path
		}
		return applyVersionLock(moduleName, "github", fullRef, version, sumFile)
	default:
		ref, version := splitRefAndVersion(right)
		return applyVersionLock(moduleName, provider, ref, version, sumFile)
	}
}

func splitRefAndVersion(input string) (string, string) {
	in := strings.TrimSpace(input)
	idx := strings.LastIndex(in, "@")
	if idx <= 0 || idx == len(in)-1 {
		return in, ""
	}
	return strings.TrimSpace(in[:idx]), strings.TrimSpace(in[idx+1:])
}

func applyVersionLock(moduleName, provider, ref, version string, sumFile *SumFile) (ResolvedModuleSource, error) {
	resolved := ResolvedModuleSource{
		Provider: strings.TrimSpace(provider),
		Ref:      strings.TrimSpace(ref),
		Version:  strings.TrimSpace(version),
	}
	_ = moduleName
	_ = sumFile
	return resolved, nil
}

func validateNonVersionedProvider(source ResolvedModuleSource) error {
	if source.Version == "" {
		return nil
	}
	if source.Provider == "self" || source.Provider == "core" {
		return fmt.Errorf("provider %q does not support version pins", source.Provider)
	}
	return nil
}

func validateSourceLock(alias string, src SourceConfig, sumFile *SumFile) error {
	if sumFile == nil {
		return nil
	}
	lock, ok := sumFile.FindSource(alias)
	if !ok {
		return nil
	}
	provider := strings.TrimSpace(src.Provider)
	if provider == "" {
		provider = alias
	}
	if p, ok := getSourceProvider(provider); ok {
		src = p.Normalize(src)
	}
	if strings.TrimSpace(lock.Provider) != provider ||
		strings.TrimSpace(lock.Path) != strings.TrimSpace(src.Path) ||
		strings.TrimSpace(lock.Repo) != strings.TrimSpace(src.Repo) ||
		strings.TrimSpace(lock.URL) != strings.TrimSpace(src.URL) {
		return fmt.Errorf("source %q lock mismatch: run `workspaced mod lock`", alias)
	}
	return nil
}

func applySourceLockOverlay(alias string, src SourceConfig, sumFile *SumFile) SourceConfig {
	if sumFile == nil {
		return src
	}
	lock, ok := sumFile.FindSource(alias)
	if !ok {
		return src
	}
	if strings.TrimSpace(src.Ref) == "" && strings.TrimSpace(lock.Ref) != "" {
		src.Ref = strings.TrimSpace(lock.Ref)
	}
	if strings.TrimSpace(src.URL) == "" && strings.TrimSpace(lock.URL) != "" {
		src.URL = strings.TrimSpace(lock.URL)
	}
	return src
}

func normalizeGitHubRepo(in string) string {
	repo := strings.Trim(strings.TrimSpace(in), "/")
	repo = strings.TrimPrefix(repo, "github:")
	repo = strings.Trim(repo, "/")
	return repo
}

func ParseSourceSpec(spec string) (SourceConfig, error) {
	trimmed := strings.TrimSpace(spec)
	if !strings.Contains(trimmed, ":") {
		return SourceConfig{}, fmt.Errorf("expected provider:target[@ref]")
	}

	ts, err := parsespec.Parse(trimmed)
	if err != nil {
		return SourceConfig{}, err
	}
	provider := strings.TrimSpace(ts.Provider)
	target := strings.TrimSpace(ts.Package)
	ref := strings.TrimSpace(ts.Version)

	cfg := SourceConfig{Provider: provider, Ref: ref}
	if !strings.Contains(trimmed, "@") {
		cfg.Ref = ""
	}
	switch provider {
	case "github":
		cfg.Repo = target
	case "local":
		cfg.Path = target
	default:
		cfg.Path = target
	}
	return cfg, nil
}
