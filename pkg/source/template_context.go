package source

import (
	"context"
	"encoding/json"
	"os"
	"workspaced/pkg/config"
	"workspaced/pkg/env"
)

func buildTemplateData(ctx context.Context, cfg *config.GlobalConfig, f File) (map[string]any, error) {
	root := map[string]any{}
	if cfg != nil {
		data, err := json.Marshal(cfg)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &root); err != nil {
			return nil, err
		}
	}

	module := map[string]any{}
	moduleName := moduleNameOf(f)
	if moduleName != "" && cfg != nil {
		if raw, ok := cfg.Modules[moduleName].(map[string]any); ok {
			module = raw
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

	if cfg != nil {
		out["Workspaces"] = cfg.Workspaces
		out["Desktop"] = cfg.Desktop
		out["Screenshot"] = cfg.Screenshot
		out["Hosts"] = cfg.Hosts
		out["Backup"] = cfg.Backup
		out["QuickSync"] = cfg.QuickSync
		out["Browser"] = cfg.Browser
		out["LazyTools"] = cfg.LazyTools
		out["Inputs"] = cfg.Inputs
		out["Modules"] = cfg.Modules
		out["Drivers"] = cfg.Drivers
	}

	return out, nil
}
