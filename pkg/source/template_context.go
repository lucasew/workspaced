package source

import (
	"context"
	"os"
	"workspaced/pkg/configcue"
	"workspaced/pkg/env"
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
	dotfilesRoot, _ := env.GetDotfilesRoot()
	userDataDir, _ := env.GetUserDataDir()

	runtime := map[string]any{
		"module_name":   moduleName,
		"dotfiles_root": dotfilesRoot,
		"home":          home,
		"user_data_dir": userDataDir,
		"is_phone":      env.IsPhone(),
		"hostname":      env.GetHostname(),
	}

	out := map[string]any{
		"root":    root,
		"module":  module,
		"runtime": runtime,
	}

	return out, nil
}
