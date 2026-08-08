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
	"workspaced/pkg/taskgroup"
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

type enabledModule struct {
	name  string
	entry configcue.ModuleEntry
}

func (p *ModuleScannerPlugin) Process(ctx context.Context, files []File) ([]File, error) {
	logger := logging.GetLogger(ctx)
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

	enabled := make([]enabledModule, 0, len(moduleNames))
	for _, modName := range moduleNames {
		modEntry := modules[modName]
		if !modEntry.Enable {
			logger.Debug("module disabled", "module", modName)
			continue
		}
		enabled = append(enabled, enabledModule{name: modName, entry: modEntry})
	}
	if len(enabled) == 0 {
		return files, nil
	}

	resolver := func(ctx context.Context, spec, base string) (string, bool, error) {
		return modFile.TryResolveSourceRefToPath(ctx, spec, base)
	}

	// Map each enabled module to its resolved files; reduce concatenates in input order.
	perModule, err := taskgroup.Map[enabledModule, []File]{
		Name:     "modules",
		Items:    enabled,
		PoolKind: taskgroup.IO,
		TaskName: func(_ int, m enabledModule) string { return "module:" + m.name },
		Fn: func(ctx context.Context, s *taskgroup.Status, m enabledModule) ([]File, error) {
			s.Update(m.name)
			return p.resolveModule(ctx, m, sumFile, resolver)
		},
	}.Run(ctx)
	if err != nil {
		return nil, err
	}

	out := files
	for _, batch := range perModule {
		out = append(out, batch...)
	}
	return out, nil
}

func (p *ModuleScannerPlugin) resolveModule(
	ctx context.Context,
	m enabledModule,
	sumFile *modfile.SumFile,
	resolver module.SourceRefResolver,
) ([]File, error) {
	logger := logging.GetLogger(ctx)
	moduleSource, err := modfile.ResolveModuleFromConfig(p.cfg, m.name, m.entry, p.baseDir, sumFile)
	if err != nil {
		return nil, fmt.Errorf("module %q: %w", m.name, err)
	}
	providerID, ref := moduleSource.Provider, moduleSource.Ref
	provider, err := module.GetProvider(providerID)
	if err != nil {
		return nil, fmt.Errorf("module %q: %w", m.name, err)
	}

	sourceSpec := providerID + ":" + ref
	if moduleSource.Version != "" {
		sourceSpec += "@" + moduleSource.Version
	}
	logger.Info("loading module", "module", m.name, "from", sourceSpec)

	// Clone so Prepare/Resolve never mutate shared CUE config from parallel workers.
	moduleConfig := cloneModuleConfig(m.entry.Config)

	if providerID == "core" {
		if cm, ok := module.GetCoreModule(ref); ok {
			if err := cm.Prepare(ctx, moduleConfig, resolver, p.baseDir); err != nil {
				return nil, fmt.Errorf("module %q: %w", m.name, err)
			}
		}
	}

	resolvedFiles, err := provider.Resolve(ctx, module.ResolveRequest{
		ModuleName:     m.name,
		Ref:            ref,
		Version:        moduleSource.Version,
		ModuleConfig:   moduleConfig,
		ModulesBaseDir: p.baseDir,
		Config:         p.cfg,
	})
	if err != nil {
		return nil, fmt.Errorf("module %q from %s:%s: %w", m.name, providerID, ref, err)
	}

	out := make([]File, 0, len(resolvedFiles))
	for _, rf := range resolvedFiles {
		fileType := TypeStatic
		if rf.Symlink {
			fileType = TypeSymlink
		}
		out = append(out, &StaticFile{
			BasicFile: BasicFile{
				RelPathStr:    rf.RelPath,
				TargetBaseDir: rf.TargetBase,
				FileMode:      rf.Mode,
				Info:          rf.Info,
				FileType:      fileType,
				Module:        m.name,
			},
			AbsPath: rf.AbsPath,
		})
	}
	return out, nil
}

// cloneModuleConfig deep-copies map[string]any trees used as module config so
// parallel Prepare steps can rewrite nested values without shared-state races.
func cloneModuleConfig(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		switch tv := v.(type) {
		case map[string]any:
			out[k] = cloneModuleConfig(tv)
		case []any:
			out[k] = cloneModuleConfigSlice(tv)
		default:
			out[k] = v
		}
	}
	return out
}

func cloneModuleConfigSlice(in []any) []any {
	out := make([]any, len(in))
	for i, v := range in {
		switch tv := v.(type) {
		case map[string]any:
			out[i] = cloneModuleConfig(tv)
		case []any:
			out[i] = cloneModuleConfigSlice(tv)
		default:
			out[i] = v
		}
	}
	return out
}
