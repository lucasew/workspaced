package source

import (
	"context"
	"os"
	"runtime"
	"workspaced/pkg/configcue"
	"workspaced/pkg/env"

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

	home, _ := os.UserHomeDir()
	dotfilesRoot, _ := env.GetDotfilesRoot(ctx)
	userDataDir, _ := env.GetUserDataDir(ctx)

	runtime := map[string]any{
		"module_name":   moduleName,
		"dotfiles_root": dotfilesRoot,
		"home":          home,
		"user_data_dir": userDataDir,
		"is_phone":      env.IsPhone(ctx),
		"hostname":      env.GetHostname(ctx),
		"goos":          runtime.GOOS,
		"goarch":        runtime.GOARCH,
		"memory":        memory.TotalMemory(),
	}

	out := map[string]any{
		"root":    root,
		"module":  module,
		"runtime": runtime,
	}

	return out, nil
}
