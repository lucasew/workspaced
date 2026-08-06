package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/git-pkgs/gitignore"
	"github.com/lucasew/workspaced/internal/module"
	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
	"github.com/lucasew/workspaced/pkg/logging"
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

// placeConfig for core:place.
//
//	items: {
//	  ".grok/skills": "mySkills:."
//	}
//	ignore_missing: true
//	steps: {
//	  "10_require": { op: "require", patterns: { skill: "SKILL.md" } }
//	  "20_demote":  { op: "move", from: "SKILL.md", to: "entry.md" }
//	}
type placeConfig struct {
	Items         map[string]string    `json:"items"`
	IgnoreMissing bool                 `json:"ignore_missing"`
	Steps         map[string]placeStep `json:"steps"`
}

type placeStep struct {
	Op       string            `json:"op"`
	From     string            `json:"from"`
	To       string            `json:"to"`
	Patterns map[string]string `json:"patterns"`
}

type placeEntry struct {
	rel     string // origin-relative, slash-separated
	abs     string
	mode    os.FileMode
	symlink bool
}

// placeStepRun is one named step applied to one item's virtual origin FS.
type placeStepRun struct {
	module string
	dest   string
	name   string
	step   placeStep
}

func (placeModule) Resolve(ctx context.Context, req module.ResolveRequest) (module.ResolveResult, error) {
	logger := logging.GetLogger(ctx)

	cfg, err := module.DecodeConfig[placeConfig](req.ModuleConfig)
	if err != nil {
		return module.ResolveResult{}, fmt.Errorf("module %s: %w", req.ModuleName, err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return module.ResolveResult{}, err
	}

	stepNames := make([]string, 0, len(cfg.Steps))
	for name := range cfg.Steps {
		stepNames = append(stepNames, name)
	}
	sort.Strings(stepNames)

	var (
		out      []module.ResolvedFile
		warnings []string
	)

	for dest, src := range cfg.Items {
		s := strings.TrimSpace(src)
		if s == "" {
			continue
		}

		srcPath := envdriver.ExpandPath(s)
		destClean := strings.Trim(dest, "/")

		st, err := os.Stat(srcPath)
		if err != nil {
			if cfg.IgnoreMissing && errors.Is(err, os.ErrNotExist) {
				logger.Info("place: skipping missing source", "module", req.ModuleName, "dest", dest, "source", srcPath)
				continue
			}
			return module.ResolveResult{}, fmt.Errorf("place source %q: %w", srcPath, err)
		}

		entries, err := collectPlaceEntries(srcPath, st)
		if err != nil {
			return module.ResolveResult{}, err
		}

		for _, name := range stepNames {
			run := placeStepRun{
				module: req.ModuleName,
				dest:   destClean,
				name:   name,
				step:   cfg.Steps[name],
			}
			var stepWarns []string
			entries, stepWarns, err = run.apply(entries)
			if err != nil {
				return module.ResolveResult{}, err
			}
			for _, w := range stepWarns {
				logger.Warn(w)
				warnings = append(warnings, w)
			}
		}

		for _, e := range entries {
			finalRel := e.rel
			if destClean != "" && destClean != "." {
				finalRel = filepath.Join(destClean, filepath.FromSlash(e.rel))
			} else {
				finalRel = filepath.FromSlash(e.rel)
			}
			out = append(out, module.ResolvedFile{
				RelPath:    finalRel,
				TargetBase: home,
				Mode:       e.mode,
				Info:       fmt.Sprintf("module:%s place (%s)", req.ModuleName, finalRel),
				AbsPath:    e.abs,
				Symlink:    e.symlink,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].RelPath < out[j].RelPath })
	return module.ResolveResult{Files: out, Warnings: warnings}, nil
}

func collectPlaceEntries(srcPath string, st os.FileInfo) ([]placeEntry, error) {
	if !st.IsDir() {
		base := filepath.Base(srcPath)
		return []placeEntry{{
			rel:     filepath.ToSlash(base),
			abs:     srcPath,
			mode:    st.Mode(),
			symlink: st.Mode()&os.ModeSymlink != 0,
		}}, nil
	}

	var entries []placeEntry
	err := filepath.Walk(srcPath, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcPath, p)
		if err != nil {
			return err
		}
		entries = append(entries, placeEntry{
			rel:     filepath.ToSlash(rel),
			abs:     p,
			mode:    info.Mode(),
			symlink: info.Mode()&os.ModeSymlink != 0,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (r placeStepRun) apply(entries []placeEntry) ([]placeEntry, []string, error) {
	// Op shape is CUE-prelude-typed; Go only dispatches known ops.
	switch strings.TrimSpace(r.step.Op) {
	case "move":
		return r.move(entries)
	case "require":
		return r.require(entries)
	default:
		return entries, nil, nil
	}
}

func (r placeStepRun) move(entries []placeEntry) ([]placeEntry, []string, error) {
	from, err := cleanPlacePath(r.step.From)
	if err != nil {
		return nil, nil, fmt.Errorf("place module %q step %q: %w: %w", r.module, r.name, errPlaceMoveFrom, err)
	}
	to, err := cleanPlacePath(r.step.To)
	if err != nil {
		return nil, nil, fmt.Errorf("place module %q step %q: %w: %w", r.module, r.name, errPlaceMoveTo, err)
	}

	prefix := from + "/"
	var (
		moving   []placeEntry
		staying  []placeEntry
		warnings []string
	)
	for _, e := range entries {
		if e.rel == from || strings.HasPrefix(e.rel, prefix) {
			moving = append(moving, e)
		} else {
			staying = append(staying, e)
		}
	}
	if len(moving) == 0 {
		msg := fmt.Sprintf("place module %q item %q step %q: move source %q missing; skipped", r.module, r.dest, r.name, from)
		return entries, []string{msg}, nil
	}

	byRel := make(map[string]placeEntry, len(staying)+len(moving))
	for _, e := range staying {
		byRel[e.rel] = e
	}

	for _, e := range moving {
		newRel := rewritePlacePrefix(e.rel, from, to)
		if prev, ok := byRel[newRel]; ok {
			msg := fmt.Sprintf("place module %q item %q step %q: move %q → %q overwrites existing %q", r.module, r.dest, r.name, e.rel, newRel, prev.abs)
			warnings = append(warnings, msg)
		}
		e.rel = newRel
		byRel[newRel] = e
	}

	out := make([]placeEntry, 0, len(byRel))
	for _, e := range byRel {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].rel < out[j].rel })
	return out, warnings, nil
}

func (r placeStepRun) require(entries []placeEntry) ([]placeEntry, []string, error) {
	// Patterns presence/shape is CUE-prelude-typed.
	names := make([]string, 0, len(r.step.Patterns))
	for name := range r.step.Patterns {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		raw := strings.TrimSpace(r.step.Patterns[name])
		if raw == "" {
			continue
		}
		negate := strings.HasPrefix(raw, "!")
		pat := raw
		if negate {
			pat = strings.TrimSpace(strings.TrimPrefix(raw, "!"))
			if pat == "" {
				continue
			}
		}

		m := gitignore.New("")
		m.AddPatterns([]byte(pat+"\n"), "")
		if errs := m.Errors(); len(errs) > 0 {
			return nil, nil, fmt.Errorf("place module %q step %q pattern %q: %w: %v", r.module, r.name, name, errPlaceBadPattern, errs[0])
		}

		matched := false
		for _, e := range entries {
			if m.MatchPath(e.rel, false) {
				matched = true
				break
			}
		}

		if negate {
			if matched {
				return nil, nil, fmt.Errorf("place module %q item %q step %q pattern %q (%s): %w", r.module, r.dest, r.name, name, raw, errPlaceRequireMustNot)
			}
			continue
		}
		if !matched {
			return nil, nil, fmt.Errorf("place module %q item %q step %q pattern %q (%s): %w", r.module, r.dest, r.name, name, raw, errPlaceRequireNoMatch)
		}
	}
	return entries, nil, nil
}

func cleanPlacePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	p = filepath.ToSlash(p)
	p = strings.Trim(p, "/")
	if p == "" || p == "." {
		return "", errPlaceEmptyPath
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return "", fmt.Errorf("%w: %q", errPlacePathEscape, p)
		}
	}
	cleaned := path.Clean(p)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("%w: %q", errPlacePathEscape, p)
	}
	return cleaned, nil
}

func rewritePlacePrefix(rel, from, to string) string {
	if rel == from {
		return to
	}
	rest := strings.TrimPrefix(rel, from+"/")
	if to == "" {
		return rest
	}
	return to + "/" + rest
}
