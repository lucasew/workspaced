package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"workspaced/pkg/configcue"
	"workspaced/pkg/env"
	"workspaced/pkg/git"
	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	parsespec "workspaced/pkg/parse/spec"
	"workspaced/pkg/tool/backend"
)

type lazyToolConfig struct {
	Version string   `json:"version"`
	Ref     string   `json:"ref"`
	Pkg     string   `json:"pkg"`
	Global  bool     `json:"global"`
	Alias   string   `json:"alias"`
	Bins    []string `json:"bins"`
}

func ResolveLazyTool(ctx context.Context, toolName, binName string) (string, error) {
	return ResolveLazyToolAt(ctx, "", toolName, binName)
}

func ResolveLazyToolAt(ctx context.Context, wd, toolName, binName string) (string, error) {
	logger := logging.GetLogger(ctx)
	currentWS, currentErr := selectLazyToolWorkspaceFrom(ctx, false, wd)
	if currentErr == nil {
		binPath, err := resolveLazyToolInWorkspace(ctx, currentWS, toolName, binName)
		if err == nil {
			return binPath, nil
		}
		logger.Debug("lazy tool resolution in current workspace failed; trying home workspace", "tool", toolName, "workspace", workspaceRootOrEmpty(currentWS), "error", err)
		currentErr = err
	}

	homeWS, homeErr := selectLazyToolWorkspaceFrom(ctx, true, wd)
	if homeErr != nil {
		if currentErr != nil {
			return "", currentErr
		}
		return "", homeErr
	}
	if workspaceRootOrEmpty(currentWS) == workspaceRootOrEmpty(homeWS) {
		return "", currentErr
	}
	logger.Debug("resolving lazy tool in home workspace fallback", "tool", toolName, "workspace", workspaceRootOrEmpty(homeWS))
	return resolveLazyToolInWorkspace(ctx, homeWS, toolName, binName)
}

func ResolveHomeLazyTool(ctx context.Context, toolName, binName string) (string, error) {
	ws, err := selectLazyToolWorkspaceFrom(ctx, true, "")
	if err != nil {
		return "", err
	}
	return resolveLazyToolInWorkspace(ctx, ws, toolName, binName)
}

func RefreshLazyToolLocks(ctx context.Context, ws *modfile.Workspace, cfg *configcue.Config) (int, error) {
	if ws == nil {
		return 0, fmt.Errorf("workspace is nil")
	}
	if cfg == nil {
		return 0, fmt.Errorf("config is nil")
	}
	if err := ws.EnsureFiles(); err != nil {
		return 0, err
	}

	sum, err := ws.LoadSumFile()
	if err != nil {
		return 0, err
	}

	lazyTools := loadLazyTools(cfg)

	names := make([]string, 0, len(lazyTools))
	for name := range lazyTools {
		names = append(names, name)
	}
	sort.Strings(names)

	updated := 0
	mgr, err := NewManager()
	if err != nil {
		return 0, err
	}
	logger := logging.GetLogger(ctx)

	for _, name := range names {
		toolCfg := lazyTools[name]
		spec, lockRef, err := lazyToolSpec(name, toolCfg)
		if err != nil {
			return 0, fmt.Errorf("lazy tool %q: %w", name, err)
		}
		if locked, ok := sum.Tool(name); ok && strings.TrimSpace(locked.Ref) == lockRef && strings.TrimSpace(locked.Version) != "" {
			continue
		}

		version := spec.Version
		if version == "" || version == "latest" {
			// Home/lazy tools: if not in lockfile, fill with latest from upstream.
			// (lockfile-driven for reproducibility; do not fall back to installed)
			logger.Info("resolving lazy tool version", "tool", name, "ref", lockRef)
			version, err = mgr.ResolveLatestVersion(ctx, spec)
			if err != nil {
				logger.Warn("failed to resolve lazy tool version", "tool", name, "ref", lockRef, "error", err)
				continue
			}
		}

		lt := lockedToolWithRenovate(lockRef, version, spec)
		changed, err := ws.UpdateSumFile(func(sum *modfile.SumFile) (bool, error) {
			return sum.EnsureTool(name, lt), nil
		})
		if err != nil {
			return 0, err
		}
		if changed {
			logger.Info("updating lazy tool lock", "tool", name, "ref", lockRef, "version", version)
			updated++
			_ = sum.EnsureTool(name, lockedToolWithRenovate(lockRef, version, spec))
		}
	}
	return updated, nil
}

type LockRefreshResult struct {
	Sources int
	Tools   int
}

func RefreshWorkspaceLocks(ctx context.Context, ws *modfile.Workspace, cfg *configcue.Config) (LockRefreshResult, error) {
	if ws == nil {
		return LockRefreshResult{}, fmt.Errorf("workspace is nil")
	}
	if cfg == nil {
		return LockRefreshResult{}, fmt.Errorf("config is nil")
	}

	lockResult, err := modfile.GenerateLockWithConfig(ctx, ws, cfg)
	if err != nil {
		return LockRefreshResult{}, err
	}
	toolLocks, err := RefreshLazyToolLocks(ctx, ws, cfg)
	if err != nil {
		return LockRefreshResult{}, err
	}

	return LockRefreshResult{
		Sources: lockResult.Sources,
		Tools:   toolLocks,
	}, nil
}

func selectLazyToolWorkspaceFrom(ctx context.Context, homeMode bool, wd string) (*modfile.Workspace, error) {
	if homeMode {
		dotfilesRoot, err := env.GetDotfilesRoot()
		if err != nil {
			return nil, fmt.Errorf("failed to get dotfiles root: %w", err)
		}
		return modfile.NewWorkspace(dotfilesRoot), nil
	}

	cwd := strings.TrimSpace(wd)
	if cwd == "" {
		cwd, _ = os.Getwd()
		return modfile.DetectWorkspace(ctx, cwd)
	}

	if abs, err := filepath.Abs(cwd); err == nil {
		cwd = abs
	}
	if root, err := git.GetRoot(ctx, cwd); err == nil && root != "" {
		return modfile.NewWorkspace(root), nil
	}
	// For explicit target directories (e.g. codebase lint path), keep lockfile local
	// even when the directory is not inside a git repository.
	return modfile.NewWorkspace(cwd), nil
}

func workspaceRootOrEmpty(ws *modfile.Workspace) string {
	if ws == nil {
		return ""
	}
	return strings.TrimSpace(ws.Root)
}

func resolveLazyToolInWorkspace(ctx context.Context, ws *modfile.Workspace, toolName, binName string) (string, error) {
	if ws == nil {
		return "", fmt.Errorf("workspace is nil")
	}
	logger := logging.GetLogger(ctx)

	cfg, err := configcue.LoadForWorkspace(ws.Root)
	if err != nil {
		return "", fmt.Errorf("failed to load workspace config: %w", err)
	}
	if err := ws.EnsureFiles(); err != nil {
		return "", err
	}

	queryToolName := toolName
	resolvedToolName, toolCfg, ok := findLazyTool(cfg, queryToolName)
	if ok {
		toolName = resolvedToolName
	}
	if !ok {
		// Allow codebase workspaces to reuse home lazy_tools while keeping lockfile local.
		if homeCfg, homeErr := configcue.LoadHome(); homeErr == nil {
			if homeToolName, homeToolCfg, homeOK := findLazyTool(homeCfg, queryToolName); homeOK {
				toolName = homeToolName
				toolCfg = homeToolCfg
				ok = true
			}
		}
	}
	if !ok {
		return "", fmt.Errorf("lazy tool %q not found in workspace or home config", toolName)
	}

	spec, lockRef, err := lazyToolSpec(toolName, toolCfg)
	if err != nil {
		return "", fmt.Errorf("invalid lazy tool spec for %q: %w", toolName, err)
	}

	sum, err := ws.LoadSumFile()
	if err != nil {
		return "", err
	}
	logger.Debug("resolving lazy tool", "tool", toolName, "workspace", ws.Root, "lockfile", ws.SumPath())

	if locked, ok := sum.Tool(toolName); ok && strings.TrimSpace(locked.Ref) == lockRef && strings.TrimSpace(locked.Version) != "" {
		spec.Version = strings.TrimSpace(locked.Version)
	}

	mgr, err := NewManager()
	if err != nil {
		return "", err
	}

	if spec.Version == "" || spec.Version == "latest" {
		// Home/lazy tools: resolved from lockfile if present.
		// If not in lockfile, fill with latest from upstream (not from installed
		// versions, to keep home tools reproducible via the lockfile).
		version, err := mgr.ResolveLatestVersion(ctx, spec)
		if err != nil {
			return "", fmt.Errorf("failed to resolve version for %q: %w", toolName, err)
		}
		spec.Version = version
	}

	lt := lockedToolWithRenovate(lockRef, spec.Version, spec)

	// Obtain the live Tool once so we can Enrich the real structure.
	var liveTool backend.Tool
	if p, err := Get(spec.Provider); err == nil {
		if t, err := p.Tool(spec.Package); err == nil {
			liveTool = t
		}
	}

	// We also directly enrich the actual *RenovateDependency inside the
	// sum's Dependencies list (the structure referenced by ref). The Tool
	// receives it by pointer via EnrichLockfile and can mutate attributes.
	// On save, any logic migrations in the Tool are applied automatically.
	if changed, err := ws.UpdateSumFile(func(sum *modfile.SumFile) (bool, error) {
		// Find or create the dep entry for this tool name.
		var dep *modfile.RenovateDependency
		for i := range sum.Dependencies {
			d := &sum.Dependencies[i]
			if d.Kind == "tool" && d.Name == toolName {
				if d.Ref == "" || d.Ref == lockRef {
					dep = d
					break
				}
			}
		}
		if dep == nil {
			sum.Dependencies = append(sum.Dependencies, modfile.RenovateDependency{
				Kind: "tool",
				Name: toolName,
			})
			dep = &sum.Dependencies[len(sum.Dependencies)-1]
		}

		// Set identity that the Tool should see (the ref is the reference key).
		dep.Ref = lockRef
		dep.Version = spec.Version
		if dep.CurrentValue == "" {
			dep.CurrentValue = spec.Version
		}

		// Pass the *actual* structure from the lockfile's Dependencies list
		// (the item referenced by ref) by pointer to the Tool.
		if liveTool != nil {
			liveTool.EnrichLockfile(dep)
		}

		// Keep the lt path too.
		_ = sum.EnsureTool(toolName, lt)
		return true, nil
	}); err != nil {
		return "", fmt.Errorf("failed to update tool lock: %w", err)
	} else if changed {
		logger.Debug("updating lazy tool lock entry", "tool", toolName, "workspace", ws.Root, "ref", lockRef, "version", spec.Version)
	} else {
		logger.Debug("lazy tool lock already up to date", "tool", toolName, "workspace", ws.Root, "ref", lockRef, "version", spec.Version)
	}

	return mgr.EnsureInstalled(ctx, spec.String(), binName)
}

func findLazyTool(cfg *configcue.Config, query string) (string, lazyToolConfig, bool) {
	lazyTools := loadLazyTools(cfg)
	if toolCfg, ok := lazyTools[query]; ok {
		return query, toolCfg, true
	}
	for name, toolCfg := range lazyTools {
		ref := strings.TrimSpace(toolCfg.Ref)
		if ref != "" && ref == query {
			return name, toolCfg, true
		}
	}
	return "", lazyToolConfig{}, false
}

func loadLazyTools(cfg *configcue.Config) map[string]lazyToolConfig {
	out := map[string]lazyToolConfig{}
	if cfg == nil {
		return out
	}
	configured := map[string]lazyToolConfig{}
	if err := cfg.Decode("lazy_tools", &configured); err != nil {
		return out
	}
	for name, toolCfg := range configured {
		out[name] = toolCfg
	}
	return out
}

func lazyToolSpec(toolName string, toolCfg lazyToolConfig) (parsespec.Spec, string, error) {
	ref := strings.TrimSpace(toolCfg.Ref)
	if ref == "" {
		ref = strings.TrimSpace(toolCfg.Pkg)
	}
	if ref == "" {
		ref = toolName
	}
	if !strings.Contains(ref, ":") {
		ref = "registry:" + ref
	}

	specStr := ref
	if !strings.Contains(specStr, "@") && strings.TrimSpace(toolCfg.Version) != "" {
		specStr += "@" + strings.TrimSpace(toolCfg.Version)
	}

	spec, err := parsespec.Parse(specStr)
	if err != nil {
		return parsespec.Spec{}, "", err
	}
	return spec, ref, nil
}

// lockedToolWithRenovate builds the lock entry. It obtains the live Tool
// and calls EnrichLockfile on a temporary RenovateDependency (the structure
// passed by reference). This gives the Tool full control to set/upgrade any
// attributes (especially renovate metadata) based on the current Ref etc.
//
// Because EnrichLockfile mutates the entry in place, any changes to the
// Tool's enrichment logic are automatically reflected in the lockfile the
// next time this tool is resolved or refreshed.
func lockedToolWithRenovate(lockRef string, version string, spec parsespec.Spec) modfile.LockedTool {
	lt := modfile.LockedTool{
		Ref:     lockRef,
		Version: version,
	}
	p, perr := Get(spec.Provider)
	if perr != nil {
		return lt
	}
	tt, terr := p.Tool(spec.Package)
	if terr != nil {
		return lt
	}

	// Create a skeleton of the actual structure that will live in the
	// lockfile's dependencies list and let the Tool mutate it directly.
	entry := modfile.RenovateDependency{
		Kind:    "tool",
		Name:    "", // the alias; set by caller context if desired
		Ref:     lockRef,
		Version: version,
	}
	tt.EnrichLockfile(&entry)

	lt.DepName = entry.DepName
	lt.Datasource = entry.Datasource
	lt.PackageName = entry.PackageName
	lt.Versioning = entry.Versioning

	// The Tool had a chance to influence CurrentValue too.
	if entry.CurrentValue != "" {
		// We don't store CurrentValue on LockedTool (it lives on the dep),
		// but the upsert logic will pick it up via other paths if needed.
	}

	return lt
}
