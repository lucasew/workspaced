package tool

import (
	"context"
	"fmt"
	"strings"
	"workspaced/pkg/config"
	"workspaced/pkg/env"
	"workspaced/pkg/modfile"
	parsespec "workspaced/pkg/parse/spec"
)

func ResolveLazyTool(ctx context.Context, toolName, binName string) (string, error) {
	cfg, err := config.LoadHome()
	if err != nil {
		return "", fmt.Errorf("failed to load home config: %w", err)
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

	specStr := ref
	if !strings.Contains(specStr, "@") && strings.TrimSpace(toolCfg.Version) != "" {
		specStr += "@" + strings.TrimSpace(toolCfg.Version)
	}

	spec, err := parsespec.Parse(specStr)
	if err != nil {
		return "", fmt.Errorf("invalid lazy tool spec for %q: %w", toolName, err)
	}

	dotfilesRoot, err := env.GetDotfilesRoot()
	if err != nil {
		return "", fmt.Errorf("failed to get dotfiles root: %w", err)
	}
	ws := modfile.NewWorkspace(dotfilesRoot)
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

	return mgr.EnsureInstalled(ctx, spec.String(), binName)
}
