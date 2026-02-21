package source

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"workspaced/pkg/config"
	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
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
	modFilePath := filepath.Join(filepath.Dir(p.baseDir), "workspaced.mod.toml")
	sumFilePath := filepath.Join(filepath.Dir(p.baseDir), "workspaced.sum.toml")
	modFile, err := modfile.LoadModFile(modFilePath)
	if err != nil {
		return nil, err
	}
	sumFile, err := modfile.LoadSumFile(sumFilePath)
	if err != nil {
		return nil, err
	}

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

		from, _ := modCfg["from"].(string) // explicit override (legacy/escape hatch)
		moduleSource, err := modFile.ResolveModuleSource(modName, from, p.baseDir, sumFile)
		if err != nil {
			return nil, fmt.Errorf("module %q: %w", modName, err)
		}
		providerID, ref := moduleSource.Provider, moduleSource.Ref
		provider, err := module.GetProvider(providerID)
		if err != nil {
			return nil, fmt.Errorf("module %q: %w", modName, err)
		}

		sourceSpec := providerID + ":" + ref
		if moduleSource.Version != "" {
			sourceSpec += "@" + moduleSource.Version
		}
		logger.Info("loading module", "module", modName, "from", sourceSpec)

		// core:base16-icons-linux keeps module source in "from=core:...",
		// while input_dir can be provided as a source ref (e.g. "papirus:Papirus").
		if providerID == "core" && ref == "base16-icons-linux" {
			if inputRef, ok := modCfg["input_dir"].(string); ok && strings.TrimSpace(inputRef) != "" {
				resolvedInputDir, resolved, err := modFile.TryResolveSourceRefToPath(inputRef, p.baseDir)
				if err != nil {
					return nil, fmt.Errorf("module %q input %q: %w", modName, inputRef, err)
				}
				if resolved {
					modCfg["input_dir"] = resolvedInputDir
				}
			}
		}

		resolvedFiles, err := provider.Resolve(ctx, module.ResolveRequest{
			ModuleName:     modName,
			Ref:            ref,
			Version:        moduleSource.Version,
			ModuleConfig:   modCfg,
			ModulesBaseDir: p.baseDir,
			Config:         p.cfg,
		})
		if err != nil {
			return nil, fmt.Errorf("module %q from %s:%s: %w", modName, providerID, ref, err)
		}

		for _, rf := range resolvedFiles {
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
