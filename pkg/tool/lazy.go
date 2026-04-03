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

		version := spec.Version
		if version == "" || version == "latest" {
			if installed, findErr := mgr.FindInstalledVersions(spec); findErr == nil && len(installed) > 0 {
				version = installed[0]
			} else {
				logger.Info("resolving lazy tool version", "tool", name, "ref", lockRef)
				version, err = mgr.ResolveLatestVersion(ctx, spec)
				if err != nil {
					logger.Warn("failed to resolve lazy tool version", "tool", name, "ref", lockRef, "error", err)
					continue
				}
			}
		}

		changed, err := modfile.UpdateSumFile(ws.SumPath(), func(sum *modfile.SumFile) (bool, error) {
			return sum.UpsertTool(name, modfile.LockedTool{
				Ref:     lockRef,
				Version: version,
			}), nil
		})
		if err != nil {
			return 0, err
		}
		if changed {
			logger.Info("updating lazy tool lock", "tool", name, "ref", lockRef, "version", version)
			updated++
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

	sum, err := modfile.LoadSumFile(ws.SumPath())
	if err != nil {
		return "", err
	}
	logger.Debug("resolving lazy tool", "tool", toolName, "workspace", ws.Root, "lockfile", ws.SumPath())

	if locked, ok := sum.FindTool(toolName); ok && strings.TrimSpace(locked.Ref) == lockRef && strings.TrimSpace(locked.Version) != "" {
		spec.Version = strings.TrimSpace(locked.Version)
	}

	mgr, err := NewManager()
	if err != nil {
		return "", err
	}

	if spec.Version == "" || spec.Version == "latest" {
		if installed, findErr := mgr.FindInstalledVersions(spec); findErr == nil && len(installed) > 0 {
			spec.Version = installed[0]
		} else {
			version, err := mgr.ResolveLatestVersion(ctx, spec)
			if err != nil {
				return "", fmt.Errorf("failed to resolve version for %q: %w", toolName, err)
			}
			spec.Version = version
		}
	}

	if changed, err := modfile.UpdateSumFile(ws.SumPath(), func(sum *modfile.SumFile) (bool, error) {
		return sum.UpsertTool(toolName, modfile.LockedTool{
			Ref:     lockRef,
			Version: spec.Version,
		}), nil
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
		ref = "mise:" + ref
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
