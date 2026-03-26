package tool

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"workspaced/pkg/config"
	"workspaced/pkg/env"
	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	parsespec "workspaced/pkg/parse/spec"
)

func ResolveLazyTool(ctx context.Context, toolName, binName string) (string, error) {
	currentWS, currentErr := selectLazyToolWorkspace(ctx, false)
	if currentErr == nil {
		binPath, err := resolveLazyToolInWorkspace(ctx, currentWS, toolName, binName)
		if err == nil {
			return binPath, nil
		}
		currentErr = err
	}

	homeWS, homeErr := selectLazyToolWorkspace(ctx, true)
	if homeErr != nil {
		if currentErr != nil {
			return "", currentErr
		}
		return "", homeErr
	}
	if workspaceRootOrEmpty(currentWS) == workspaceRootOrEmpty(homeWS) {
		return "", currentErr
	}
	return resolveLazyToolInWorkspace(ctx, homeWS, toolName, binName)
}

func ResolveHomeLazyTool(ctx context.Context, toolName, binName string) (string, error) {
	ws, err := selectLazyToolWorkspace(ctx, true)
	if err != nil {
		return "", err
	}
	return resolveLazyToolInWorkspace(ctx, ws, toolName, binName)
}

func RefreshLazyToolLocks(ctx context.Context, ws *modfile.Workspace, cfg *config.GlobalConfig) (int, error) {
	if ws == nil {
		return 0, fmt.Errorf("workspace is nil")
	}
	if cfg == nil {
		return 0, fmt.Errorf("config is nil")
	}
	if err := ws.EnsureFiles(); err != nil {
		return 0, err
	}

	sum, err := modfile.LoadSumFile(ws.SumPath())
	if err != nil {
		return 0, err
	}
	beforeTools := len(sum.Tools)

	names := make([]string, 0, len(cfg.LazyTools))
	for name := range cfg.LazyTools {
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
		toolCfg := cfg.LazyTools[name]
		spec, lockRef, err := lazyToolSpec(name, toolCfg)
		if err != nil {
			return 0, fmt.Errorf("lazy tool %q: %w", name, err)
		}

		if locked, ok := sum.Tools[name]; ok && strings.TrimSpace(locked.Ref) == lockRef && strings.TrimSpace(locked.Version) != "" {
			continue
		}

		version := spec.Version
		if version == "" || version == "latest" {
			logger.Info("resolving lazy tool version", "tool", name, "ref", lockRef)
			version, err = mgr.ResolveLatestVersion(ctx, spec)
			if err != nil {
				logger.Warn("failed to resolve lazy tool version", "tool", name, "ref", lockRef, "error", err)
				continue
			}
		}

		if current := sum.Tools[name]; current.Ref != lockRef || current.Version != version {
			logger.Info("updating lazy tool lock", "tool", name, "ref", lockRef, "version", version)
			sum.Tools[name] = modfile.LockedTool{Ref: lockRef, Version: version}
			updated++
		}
	}

	if updated == 0 {
		return 0, nil
	}
	if len(sum.Tools) < beforeTools {
		return 0, fmt.Errorf("refusing to shrink tool lock entries: before=%d after=%d", beforeTools, len(sum.Tools))
	}
	if err := modfile.WriteSumFile(ws.SumPath(), sum); err != nil {
		return 0, err
	}
	return updated, nil
}

type LockRefreshResult struct {
	Sources int
	Tools   int
}

func RefreshWorkspaceLocks(ctx context.Context, ws *modfile.Workspace, cfg *config.GlobalConfig) (LockRefreshResult, error) {
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

func selectLazyToolWorkspace(ctx context.Context, homeMode bool) (*modfile.Workspace, error) {
	if homeMode {
		dotfilesRoot, err := env.GetDotfilesRoot()
		if err != nil {
			return nil, fmt.Errorf("failed to get dotfiles root: %w", err)
		}
		return modfile.NewWorkspace(dotfilesRoot), nil
	}

	cwd, _ := os.Getwd()
	return modfile.DetectWorkspace(ctx, cwd)
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

	cfg, err := config.LoadForWorkspace(ws.Root)
	if err != nil {
		return "", fmt.Errorf("failed to load workspace config: %w", err)
	}
	if _, err := RefreshWorkspaceLocks(ctx, ws, cfg.GlobalConfig); err != nil {
		return "", fmt.Errorf("failed to refresh workspace lockfile: %w", err)
	}

	toolCfg, ok := cfg.LazyTools[toolName]
	if !ok {
		return "", fmt.Errorf("lazy tool %q not found in config", toolName)
	}

	spec, lockRef, err := lazyToolSpec(toolName, toolCfg)
	if err != nil {
		return "", fmt.Errorf("invalid lazy tool spec for %q: %w", toolName, err)
	}

	sum, err := modfile.LoadSumFile(ws.SumPath())
	if err != nil {
		return "", err
	}

	if locked, ok := sum.Tools[toolName]; ok && strings.TrimSpace(locked.Ref) == lockRef && strings.TrimSpace(locked.Version) != "" {
		spec.Version = strings.TrimSpace(locked.Version)
	}

	mgr, err := NewManager()
	if err != nil {
		return "", err
	}

	if spec.Version == "" || spec.Version == "latest" {
		version, err := mgr.ResolveLatestVersion(ctx, spec)
		if err != nil {
			return "", fmt.Errorf("failed to resolve version for %q: %w", toolName, err)
		}
		spec.Version = version
	}

	if current := sum.Tools[toolName]; current.Ref != lockRef || current.Version != spec.Version {
		sum.Tools[toolName] = modfile.LockedTool{
			Ref:     lockRef,
			Version: spec.Version,
		}
		if err := modfile.WriteSumFile(ws.SumPath(), sum); err != nil {
			return "", fmt.Errorf("failed to update tool lock: %w", err)
		}
	}

	return mgr.EnsureInstalled(ctx, spec.String(), binName)
}

func lazyToolSpec(toolName string, toolCfg config.LazyToolConfig) (parsespec.Spec, string, error) {
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
