package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	envdriver "workspaced/pkg/driver/env"
	"workspaced/pkg/module"
)

func init() {
	module.RegisterCoreModule(placeModule{})
}

type placeModule struct{}

func (placeModule) Ref() string { return "place" }

func (placeModule) Prepare(ctx context.Context, cfg map[string]any, resolver module.SourceRefResolver, modulesBaseDir string) error {
	if raw, ok := cfg["items"]; ok {
		if items, ok := raw.(map[string]any); ok {
			for dest, v := range items {
				if s, ok := v.(string); ok {
					resolved, did, err := resolver(ctx, s, modulesBaseDir)
					if err != nil {
						return fmt.Errorf("items[%q]: %w", dest, err)
					}
					if did {
						items[dest] = resolved
					}
				}
			}
		}
	}
	return nil
}

// placeConfig for core:place. One shape only:
//
//	items: {
//	  ".grok/skills": "mySkills:."
//	  ".config/agent/prompts": "mySkills:prompts"
//	  // You can also reference the built-in self input directly:
//	  ".local/bin": "self:bin"
//	}
type placeConfig struct {
	Items map[string]string `json:"items"`
}

func (placeModule) Resolve(ctx context.Context, req module.ResolveRequest) ([]module.ResolvedFile, error) {
	_ = ctx

	cfg, err := module.DecodeConfig[placeConfig](req.ModuleConfig)
	if err != nil {
		return nil, fmt.Errorf("module %s: %w", req.ModuleName, err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	var out []module.ResolvedFile

	for dest, src := range cfg.Items {
		s := strings.TrimSpace(src)
		if s == "" {
			continue
		}

		srcPath := envdriver.ExpandPath(s)
		destClean := strings.Trim(dest, "/")

		st, err := os.Stat(srcPath)
		if err != nil {
			return nil, fmt.Errorf("place source %q: %w", srcPath, err)
		}

		if !st.IsDir() {
			// single file
			base := filepath.Base(srcPath)
			finalRel := base
			if destClean != "" && destClean != "." {
				finalRel = filepath.Join(destClean, base)
			}
			isSymlink := st.Mode()&os.ModeSymlink != 0
			out = append(out, module.ResolvedFile{
				RelPath:    finalRel,
				TargetBase: home,
				Mode:       st.Mode(),
				Info:       fmt.Sprintf("module:%s place (%s)", req.ModuleName, finalRel),
				AbsPath:    srcPath,
				Symlink:    isSymlink,
			})
			continue
		}

		// directory tree
		err = filepath.Walk(srcPath, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(srcPath, path)
			if err != nil {
				return err
			}
			finalRel := rel
			if destClean != "" && destClean != "." {
				finalRel = filepath.Join(destClean, rel)
			}
			isSymlink := info.Mode()&os.ModeSymlink != 0
			out = append(out, module.ResolvedFile{
				RelPath:    finalRel,
				TargetBase: home,
				Mode:       info.Mode(),
				Info:       fmt.Sprintf("module:%s place (%s)", req.ModuleName, finalRel),
				AbsPath:    path,
				Symlink:    isSymlink,
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].RelPath < out[j].RelPath })
	return out, nil
}
