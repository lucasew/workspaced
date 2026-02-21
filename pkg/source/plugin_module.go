package source

import (
	"context"
	"fmt"
	"sort"
	"workspaced/pkg/config"
	"workspaced/pkg/logging"
	"workspaced/pkg/module"
	_ "workspaced/pkg/module/prelude"
)

type ModuleScannerPlugin struct {
	baseDir  string
	cfg      *config.GlobalConfig
	priority int
}

func NewModuleScannerPlugin(baseDir string, cfg *config.GlobalConfig, priority int) *ModuleScannerPlugin {
	return &ModuleScannerPlugin{
		baseDir:  baseDir,
		cfg:      cfg,
		priority: priority,
	}
}

func (p *ModuleScannerPlugin) Name() string {
	return "module-scanner"
}

func (p *ModuleScannerPlugin) Process(ctx context.Context, files []File) ([]File, error) {
	logger := logging.GetLogger(ctx)
	discovered := []File{}

	moduleNames := make([]string, 0, len(p.cfg.Modules))
	for name := range p.cfg.Modules {
		moduleNames = append(moduleNames, name)
	}
	sort.Strings(moduleNames)

	for _, modName := range moduleNames {
		modCfgRaw := p.cfg.Modules[modName]
		if modCfgRaw == nil {
			continue
		}
		modCfg, ok := modCfgRaw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid config for module %q: expected map, got %T", modName, modCfgRaw)
		}
		enabled, _ := modCfg["enable"].(bool)
		if !enabled {
			logger.Debug("module disabled", "module", modName)
			continue
		}

		from, _ := modCfg["from"].(string)
		providerID, ref, err := module.ResolveProviderAndRef(from, modName)
		if err != nil {
			return nil, fmt.Errorf("module %q: %w", modName, err)
		}
		provider, err := module.GetProvider(providerID)
		if err != nil {
			return nil, fmt.Errorf("module %q: %w", modName, err)
		}

		logger.Info("loading module", "module", modName, "from", providerID+":"+ref)
		resolved, err := provider.Resolve(ctx, module.ResolveRequest{
			ModuleName:     modName,
			Ref:            ref,
			ModuleConfig:   modCfg,
			ModulesBaseDir: p.baseDir,
			Config:         p.cfg,
		})
		if err != nil {
			return nil, fmt.Errorf("module %q from %s:%s: %w", modName, providerID, ref, err)
		}

		for _, rf := range resolved {
			fileType := TypeStatic
			if rf.Symlink {
				fileType = TypeSymlink
			}
			discovered = append(discovered, &StaticFile{
				BasicFile: BasicFile{
					RelPathStr:    rf.RelPath,
					TargetBaseDir: rf.TargetBase,
					FileMode:      rf.Mode,
					Info:          rf.Info,
					FileType:      fileType,
				},
				AbsPath: rf.AbsPath,
			})
		}
	}

	return append(files, discovered...), nil
}
