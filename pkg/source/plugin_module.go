package source

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
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

		// core:base16-icons-linux keeps module source in "from=core:...",
		// while input_dir can be provided as a source ref (e.g. "papirus:Papirus").
		if providerID == "core" && ref == "base16-icons-linux" {
			if inputRef, ok := moduleConfig["input_dir"].(string); ok && strings.TrimSpace(inputRef) != "" {
				resolvedInputDir, resolved, err := modFile.TryResolveSourceRefToPath(ctx, inputRef, p.baseDir)
				if err != nil {
					return nil, fmt.Errorf("module %q input %q: %w", modName, inputRef, err)
				}
				if resolved {
					moduleConfig["input_dir"] = resolvedInputDir
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
