package tool

import (
	"context"
	"fmt"
	"os"
	"strings"
	"workspaced/pkg/config"
	"workspaced/pkg/env"
	"workspaced/pkg/miseutil"
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

	toolCfg, ok := cfg.LazyTools[toolName]
	if !ok {
		return "", fmt.Errorf("lazy tool %q not found in config", toolName)
	}

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
		return "", fmt.Errorf("invalid lazy tool spec for %q: %w", toolName, err)
	}

	if err := ws.EnsureFiles(); err != nil {
		return "", err
	}

	sum, err := modfile.LoadSumFile(ws.SumPath())
	if err != nil {
		return "", err
	}

	if locked, ok := sum.Tools[toolName]; ok && strings.TrimSpace(locked.Ref) == ref && strings.TrimSpace(locked.Version) != "" {
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

	if current := sum.Tools[toolName]; current.Ref != ref || current.Version != spec.Version {
		sum.Tools[toolName] = modfile.LockedTool{
			Ref:     ref,
			Version: spec.Version,
		}
		if err := modfile.WriteSumFile(ws.SumPath(), sum); err != nil {
			return "", fmt.Errorf("failed to update tool lock: %w", err)
		}
	}
	if spec.Provider == "mise" {
		toolSpec := spec.Package + "@" + spec.Version
		binPath, err := miseutil.ResolveBinPath(ctx, binName, toolSpec)
		if err == nil && strings.TrimSpace(binPath) != "" {
			return strings.TrimSpace(binPath), nil
		}
		if err := miseutil.Run(ctx, "install", toolSpec); err != nil {
			return "", err
		}
		return miseutil.ResolveBinPath(ctx, binName, toolSpec)
	}

	return mgr.EnsureInstalled(ctx, spec.String(), binName)
}
