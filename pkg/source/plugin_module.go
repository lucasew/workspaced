package source

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"workspaced/pkg/configcue"
	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	_ "workspaced/pkg/modfile/sourceprovider/prelude"
	"workspaced/pkg/module"
	_ "workspaced/pkg/module/prelude"
)

type ModuleScannerPlugin struct {
	baseDir  string
	cfg      *configcue.Config
	priority int
}

func NewModuleScannerPlugin(baseDir string, cfg *configcue.Config, priority int) *ModuleScannerPlugin {
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
	sumFilePath := filepath.Join(filepath.Dir(p.baseDir), "workspaced.lock.json")
	modFile, err := modfile.ModFileFromConfig(p.cfg)
	if err != nil {
		return nil, err
	}
	sumFile, err := modfile.LoadSumFile(sumFilePath)
	if err != nil {
		return nil, err
	}

	modules, err := p.cfg.Modules()
	if err != nil {
		return nil, err
	}
	moduleNames := make([]string, 0, len(modules))
	for name := range modules {
		moduleNames = append(moduleNames, name)
	}
	sort.Strings(moduleNames)

	for _, modName := range moduleNames {
		modEntry := modules[modName]
		if !modEntry.Enable {
			logger.Debug("module disabled", "module", modName)
			continue
		}

		moduleSource, err := modfile.ResolveModuleFromConfig(p.cfg, modName, modEntry, p.baseDir, sumFile)
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

		moduleConfig := modEntry.Config
		if moduleConfig == nil {
			moduleConfig = map[string]any{}
		}

		// Give registered core modules a chance to rewrite their config
		// (e.g. resolve "alias:path" source refs to real directories).
		if providerID == "core" {
			if cm, ok := module.GetCoreModule(ref); ok {
				resolver := func(ctx context.Context, spec, base string) (string, bool, error) {
					return modFile.TryResolveSourceRefToPath(ctx, spec, base)
				}
				if err := cm.Prepare(ctx, moduleConfig, resolver, p.baseDir); err != nil {
					return nil, fmt.Errorf("module %q: %w", modName, err)
				}
			}
		}

		resolvedFiles, err := provider.Resolve(ctx, module.ResolveRequest{
			ModuleName:     modName,
			Ref:            ref,
			Version:        moduleSource.Version,
			ModuleConfig:   moduleConfig,
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
					Module:        modName,
				},
				AbsPath: rf.AbsPath,
			})
		}
	}

	return append(files, discovered...), nil
}
