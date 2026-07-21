package source

import (
	"context"
	"fmt"
	"runtime"

	"github.com/lucasew/workspaced/internal/configcue"
	envdriver "github.com/lucasew/workspaced/pkg/driver/env"

	"github.com/pbnjay/memory"
)

func buildTemplateData(ctx context.Context, cfg *configcue.Config, f File) (map[string]any, error) {
	root := map[string]any{}
	if cfg != nil {
		root = cfg.Raw()
	}

	module := map[string]any{}
	moduleName := moduleNameOf(f)
	if moduleName != "" && cfg != nil {
		if raw, err := cfg.Lookup("modules." + moduleName + ".config"); err == nil {
			if mapped, ok := raw.(map[string]any); ok {
				module = mapped
			}
		}
		if len(module) == 0 {
			if raw, err := cfg.Lookup("modules." + moduleName); err == nil {
				if mapped, ok := raw.(map[string]any); ok {
					module = mapped
				}
			}
		}
	}

	home, err := envdriver.GetHomeDir(ctx)
	if err != nil {
		home, err = envdriver.ResolveHomeDir()
		if err != nil {
			return nil, fmt.Errorf("user home dir: %w", err)
		}
	}
	userDataDir, err := envdriver.GetUserDataDir(ctx)
	if err != nil {
		return nil, fmt.Errorf("user data dir: %w", err)
	}
	hostname, err := envdriver.GetHostname(ctx)
	if err != nil {
		return nil, fmt.Errorf("hostname: %w", err)
	}

	runtimeData := map[string]any{
		"module_name":   moduleName,
		"home":          home,
		"user_data_dir": userDataDir,
		"is_phone":      envdriver.IsPhone(ctx),
		"hostname":      hostname,
		"goos":          runtime.GOOS,
		"goarch":        runtime.GOARCH,
		"memory":        memory.TotalMemory(),
	}
	if dotfilesRoot, err := envdriver.GetDotfilesRoot(ctx); err == nil && dotfilesRoot != "" {
		runtimeData["dotfiles_root"] = dotfilesRoot
	}

	out := map[string]any{
		"root":    root,
		"module":  module,
		"runtime": runtimeData,
	}

	return out, nil
}
