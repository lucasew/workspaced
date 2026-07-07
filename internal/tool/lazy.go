package tool

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"workspaced/internal/configcue"
	"workspaced/internal/git"
	"workspaced/internal/modfile"
	parsespec "workspaced/internal/parse/spec"
	"workspaced/internal/tool/backend"
	envdriver "workspaced/pkg/driver/env"
	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"
)

var (
	// ErrNilWorkspace is returned when a nil workspace is passed to a function that requires one.
	ErrNilWorkspace = errors.New("workspace is nil")
	// ErrNilConfig is returned when a nil config is passed to a function that requires one.
	ErrNilConfig = errors.New("config is nil")
	// ErrLazyToolNotFound is returned when a lazy tool alias cannot be found in the workspace or home config.
	ErrLazyToolNotFound = errors.New("lazy tool not found in workspace or home config")
)

type lazyToolConfig struct {
	Version string   `json:"version"`
	Ref     string   `json:"ref"`
	Pkg     string   `json:"pkg"`
	Global  bool     `json:"global"`
	Alias   string   `json:"alias"`
	Bins    []string `json:"bins"`
}

// ResolveLazyTool maps an abstract tool alias (e.g., "fmt") to a localized binary path,
// respecting the configuration context of the current working directory or falling back to home.
// It uses an empty working directory to trigger auto-detection of the workspace.
func ResolveLazyTool(ctx context.Context, toolName, binName string) (string, error) {
	return ResolveLazyToolAt(ctx, "", toolName, binName)
}

// ResolveLazyToolAt binds an abstract alias to an executable path using the explicit
// working directory to anchor workspace detection. If the localized workspace lacks
// the tool, it cascades resolution to the global dotfiles/home workspace context.
func ResolveLazyToolAt(ctx context.Context, wd, toolName, binName string) (string, error) {
	logger := logging.GetLogger(ctx)
	currentWS, currentErr := selectLazyToolWorkspaceFrom(ctx, false, wd)
	if currentErr == nil {
		binPath, err := resolveLazyToolInWorkspace(ctx, currentWS, toolName, binName)
		if err == nil {
			return binPath, nil
		}
		logger.Debug("lazy tool resolution in current workspace failed; trying home workspace", "tool", toolName, "workspace", workspaceRootOrEmpty(currentWS), "error", err)
		currentErr = err
	}

	homeWS, homeErr := selectLazyToolWorkspaceFrom(ctx, true, wd)
	if homeErr != nil {
		if currentErr != nil {
			return "", currentErr
		}
		return "", homeErr
	}
	if workspaceRootOrEmpty(currentWS) == workspaceRootOrEmpty(homeWS) {
		return "", currentErr
	}
	logger.Debug("resolving lazy tool in home workspace fallback", "tool", toolName, "workspace", workspaceRootOrEmpty(homeWS))
	return resolveLazyToolInWorkspace(ctx, homeWS, toolName, binName)
}

// ResolveHomeLazyTool forces resolution of a lazy tool explicitly within the context
// of the global user dotfiles workspace, bypassing local workspace overrides.
func ResolveHomeLazyTool(ctx context.Context, toolName, binName string) (string, error) {
	ws, err := selectLazyToolWorkspaceFrom(ctx, true, "")
	if err != nil {
		return "", err
	}
	return resolveLazyToolInWorkspace(ctx, ws, toolName, binName)
}

// RefreshLazyToolLocks synchronizes the configuration mapping of lazy tools with the
// modfile lockfile, resolving latest versions if missing, and enriching lock metadata
// to ensure CI reproducible executions. Returns the count of updated tool definitions.
func RefreshLazyToolLocks(ctx context.Context, ws *modfile.Workspace, cfg *configcue.Config) (int, error) {
	if ws == nil {
		return 0, ErrNilWorkspace
	}
	if cfg == nil {
		return 0, ErrNilConfig
	}
	if err := ws.EnsureFiles(ctx); err != nil {
		return 0, err
	}

	sum, err := ws.LoadSumFile()
	if err != nil {
		return 0, err
	}

	lazyTools := loadLazyTools(cfg)

	names := make([]string, 0, len(lazyTools))
	for name := range lazyTools {
		names = append(names, name)
	}
	sort.Strings(names)

	updated := 0
	mgr, err := NewManager()
	if err != nil {
		return 0, err
	}
	logger := logging.GetLogger(ctx)

	// Collect tools that actually need work (no good version locked yet).
	needsWork := make([]string, 0, len(names))
	for _, name := range names {
		toolCfg := lazyTools[name]
		_, lockRef, err := lazyToolSpec(name, toolCfg)
		if err != nil {
			return 0, fmt.Errorf("lazy tool %q: %w", name, err)
		}
		if locked, ok := sum.Tool(lockRef); ok && strings.TrimSpace(locked.Ref) == lockRef && strings.TrimSpace(locked.Version) != "" {
			continue
		}
		needsWork = append(needsWork, name)
	}

	if len(needsWork) == 0 {
		return 0, nil
	}

	type update struct {
		name string
		lt   modfile.LockedTool
	}
	// Map resolves versions in parallel; reduce writes the lockfile serially.
	updates, err := taskgroup.Map[string, update]{
		Name:     "lazy-tools",
		Items:    needsWork,
		PoolKind: taskgroup.Control,
		TaskName: func(_ int, name string) string { return "tool:" + name },
		Fn: func(ctx context.Context, s *taskgroup.Status, name string) (update, error) {
			toolCfg := lazyTools[name]
			spec, lockRef, err := lazyToolSpec(name, toolCfg)
			if err != nil {
				return update{}, err
			}
			version := spec.Version
			if version == "" || version == "latest" {
				s.Update("resolving latest for " + name)
				l := logging.GetLogger(ctx)
				l.Info("resolving lazy tool version", "tool", name, "ref", lockRef)
				v, err := mgr.ResolveLatestVersion(ctx, spec)
				if err != nil {
					return update{}, fmt.Errorf("resolve latest for %q: %w", name, err)
				}
				version = v
			} else {
				s.Update("preparing lock entry for " + name)
			}
			s.Update(name + "@" + version)
			return update{name: name, lt: lockedToolWithRenovate(lockRef, version, spec)}, nil
		},
	}.Run(ctx)
	if err != nil {
		return 0, err
	}

	// Reduce: apply collected updates serially (lockfile mutation must be safe).
	for _, u := range updates {
		name := u.name
		lt := u.lt
		lockRef := lt.Ref
		version := lt.Version

		changed, err := ws.UpdateSumFile(ctx, func(sum *modfile.SumFile) (bool, error) {
			return sum.EnsureTool(name, lt), nil
		})
		if err != nil {
			return 0, err
		}
		if changed {
			logger.Info("updating lazy tool lock", "tool", name, "ref", lockRef, "version", version)
			updated++
			// Mirror the local sum copy so it reflects the write (harmless at end of function,
			// kept for behavioral parity with the original sequential implementation).
			_ = sum.EnsureTool(name, lt)
		}
	}
	return updated, nil
}

// LockRefreshResult captures the volume of locking state mutated during a refresh pass.
type LockRefreshResult struct {
	Sources int
	Tools   int
	Changed bool
}

// RefreshWorkspaceLocks orchestrates a full update of both source (dependency) locks
// and tool (execution) locks, effectively aligning the workspace modfile with the configured states.
func RefreshWorkspaceLocks(ctx context.Context, ws *modfile.Workspace, cfg *configcue.Config) (LockRefreshResult, error) {
	if ws == nil {
		return LockRefreshResult{}, ErrNilWorkspace
	}
	if cfg == nil {
		return LockRefreshResult{}, ErrNilConfig
	}
	logger := logging.GetLogger(ctx)

	lockResult, err := modfile.GenerateLockWithConfig(ctx, ws, cfg, false)
	if err != nil {
		return LockRefreshResult{}, err
	}
	toolLocks, err := RefreshLazyToolLocks(ctx, ws, cfg)
	if err != nil {
		return LockRefreshResult{}, err
	}

	result := LockRefreshResult{
		Sources: lockResult.Sources,
		Tools:   toolLocks,
		Changed: lockResult.Changed || toolLocks > 0,
	}
	if result.Changed {
		logger.Info("workspace lockfile updated", "path", ws.SumPath(), "sources", result.Sources, "tools", result.Tools)
	} else {
		logger.Info("workspace lockfile unchanged", "path", ws.SumPath(), "sources", result.Sources, "tools", result.Tools)
	}
	return result, nil
}

func selectLazyToolWorkspaceFrom(ctx context.Context, homeMode bool, wd string) (*modfile.Workspace, error) {
	if homeMode {
		dotfilesRoot, err := envdriver.GetDotfilesRoot(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get dotfiles root: %w", err)
		}
		return modfile.NewWorkspace(dotfilesRoot), nil
	}

	cwd := strings.TrimSpace(wd)
	if cwd == "" {
		cwd, _ = os.Getwd()
		return modfile.DetectWorkspace(ctx, cwd)
	}

	if abs, err := filepath.Abs(cwd); err == nil {
		cwd = abs
	}
	if root, err := git.GetRoot(ctx, cwd); err == nil && root != "" {
		return modfile.NewWorkspace(root), nil
	}
	// For explicit target directories (e.g. codebase lint path), keep lockfile local
	// even when the directory is not inside a git repository.
	return modfile.NewWorkspace(cwd), nil
}

func workspaceRootOrEmpty(ws *modfile.Workspace) string {
	if ws == nil {
		return ""
	}
	return strings.TrimSpace(ws.Root)
}

func resolveLazyToolInWorkspace(ctx context.Context, ws *modfile.Workspace, toolName, binName string) (string, error) {
	if ws == nil {
		return "", ErrNilWorkspace
	}
	logger := logging.GetLogger(ctx)

	cfg, err := configcue.LoadForWorkspace(ctx, ws.Root)
	if err != nil {
		return "", fmt.Errorf("failed to load workspace config: %w", err)
	}
	if err := ws.EnsureFiles(ctx); err != nil {
		return "", err
	}

	queryToolName := toolName
	resolvedToolName, toolCfg, ok := findLazyTool(cfg, queryToolName)
	if ok {
		toolName = resolvedToolName
	}
	if !ok {
		// Allow codebase workspaces to reuse home lazy_tools while keeping lockfile local.
		if homeCfg, homeErr := configcue.LoadHome(ctx); homeErr == nil {
			if homeToolName, homeToolCfg, homeOK := findLazyTool(homeCfg, queryToolName); homeOK {
				toolName = homeToolName
				toolCfg = homeToolCfg
				ok = true
			}
		}
	}
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrLazyToolNotFound, toolName)
	}

	spec, lockRef, err := lazyToolSpec(toolName, toolCfg)
	if err != nil {
		return "", fmt.Errorf("invalid lazy tool spec for %q: %w", toolName, err)
	}

	sum, err := ws.LoadSumFile()
	if err != nil {
		return "", err
	}
	logger.Debug("resolving lazy tool", "tool", toolName, "workspace", ws.Root, "lockfile", ws.SumPath())

	if locked, ok := sum.Tool(lockRef); ok && strings.TrimSpace(locked.Ref) == lockRef && strings.TrimSpace(locked.Version) != "" {
		spec.Version = strings.TrimSpace(locked.Version)
	}

	mgr, err := NewManager()
	if err != nil {
		return "", err
	}

	if spec.Version == "" || spec.Version == "latest" {
		// Home/lazy tools: resolved from lockfile if present.
		// If not in lockfile, fill with latest from upstream (not from installed
		// versions, to keep home tools reproducible via the lockfile).
		version, err := mgr.ResolveLatestVersion(ctx, spec)
		if err != nil {
			return "", fmt.Errorf("failed to resolve version for %q: %w", toolName, err)
		}
		spec.Version = version
	}

	lt := lockedToolWithRenovate(lockRef, spec.Version, spec)

	// Obtain the live Tool once so we can Enrich the real structure.
	var liveTool backend.Tool
	if p, err := Get(spec.Provider); err == nil {
		if t, err := p.Tool(spec.Package); err == nil {
			liveTool = t
		}
	}

	// Enrich the live RenovateDependency row and mirror into EnsureTool.
	// Only rewrite the lockfile when fields actually change.
	if changed, err := ws.UpdateSumFile(ctx, func(sum *modfile.SumFile) (bool, error) {
		changed := applyLiveToolEnrichment(sum, lockRef, spec.Version, liveTool)
		if sum.EnsureTool(toolName, lt) {
			changed = true
		}
		return changed, nil
	}); err != nil {
		return "", fmt.Errorf("failed to update tool lock: %w", err)
	} else if changed {
		logger.Debug("updating lazy tool lock entry", "tool", toolName, "workspace", ws.Root, "ref", lockRef, "version", spec.Version)
	} else {
		logger.Debug("lazy tool lock already up to date", "tool", toolName, "workspace", ws.Root, "ref", lockRef, "version", spec.Version)
	}

	return mgr.EnsureInstalled(ctx, spec.String(), binName)
}

func findLazyTool(cfg *configcue.Config, query string) (string, lazyToolConfig, bool) {
	lazyTools := loadLazyTools(cfg)
	if toolCfg, ok := lazyTools[query]; ok {
		return query, toolCfg, true
	}
	for name, toolCfg := range lazyTools {
		ref := strings.TrimSpace(toolCfg.Ref)
		if ref != "" && ref == query {
			return name, toolCfg, true
		}
	}
	return "", lazyToolConfig{}, false
}

func loadLazyTools(cfg *configcue.Config) map[string]lazyToolConfig {
	out := map[string]lazyToolConfig{}
	if cfg == nil {
		return out
	}
	configured := map[string]lazyToolConfig{}
	if err := cfg.Decode("lazy_tools", &configured); err != nil {
		return out
	}
	for name, toolCfg := range configured {
		out[name] = toolCfg
	}
	return out
}

func lazyToolSpec(toolName string, toolCfg lazyToolConfig) (parsespec.Spec, string, error) {
	ref := strings.TrimSpace(toolCfg.Ref)
	if ref == "" {
		ref = strings.TrimSpace(toolCfg.Pkg)
	}
	if ref == "" {
		ref = toolName
	}
	if !strings.Contains(ref, ":") {
		ref = "registry:" + ref
	}

	specStr := ref
	if !strings.Contains(specStr, "@") && strings.TrimSpace(toolCfg.Version) != "" {
		specStr += "@" + strings.TrimSpace(toolCfg.Version)
	}

	spec, err := parsespec.Parse(specStr)
	if err != nil {
		return parsespec.Spec{}, "", err
	}
	return spec, ref, nil
}

// applyLiveToolEnrichment finds the tool row keyed by lockRef (creating it
// if missing), runs Tool.EnrichLockfile on that live struct, and reports
// whether any persisted field changed.
func applyLiveToolEnrichment(sum *modfile.SumFile, lockRef, version string, liveTool backend.Tool) bool {
	if sum == nil {
		return false
	}
	lockRef = strings.TrimSpace(lockRef)
	version = strings.TrimSpace(version)
	if lockRef == "" {
		return false
	}

	var dep *modfile.RenovateDependency
	for i := range sum.Dependencies {
		d := &sum.Dependencies[i]
		if d.Kind == "tool" && strings.TrimSpace(d.Ref) == lockRef {
			dep = d
			break
		}
	}
	created := false
	if dep == nil {
		sum.Dependencies = append(sum.Dependencies, modfile.RenovateDependency{
			Kind: "tool",
			Ref:  lockRef,
		})
		dep = &sum.Dependencies[len(sum.Dependencies)-1]
		created = true
	}

	before := *dep
	dep.Kind = "tool"
	dep.Ref = lockRef
	if strings.TrimSpace(dep.CurrentValue) == "" && version != "" {
		dep.CurrentValue = version
	}
	if liveTool != nil {
		liveTool.EnrichLockfile(dep)
	}
	return created || !renovateDependencyEqual(before, *dep)
}

func renovateDependencyEqual(a, b modfile.RenovateDependency) bool {
	if a.Kind != b.Kind || a.Ref != b.Ref || a.DepName != b.DepName ||
		a.PackageName != b.PackageName || a.CurrentValue != b.CurrentValue ||
		a.CurrentDigest != b.CurrentDigest || a.CurrentVersion != b.CurrentVersion ||
		a.Datasource != b.Datasource || a.Versioning != b.Versioning ||
		a.ExtractVersion != b.ExtractVersion || a.DepType != b.DepType ||
		a.SourceUrl != b.SourceUrl || a.Manager != b.Manager || a.SkipReason != b.SkipReason {
		return false
	}
	if len(a.RegistryUrls) != len(b.RegistryUrls) {
		return false
	}
	for i := range a.RegistryUrls {
		if a.RegistryUrls[i] != b.RegistryUrls[i] {
			return false
		}
	}
	return true
}

// lockedToolWithRenovate builds the lock entry. It obtains the live Tool
// and calls EnrichLockfile on a temporary RenovateDependency (the structure
// passed by reference). This gives the Tool full control to set/upgrade any
// attributes (especially renovate metadata) based on the current Ref etc.
//
// Because EnrichLockfile mutates the entry in place, any changes to the
// Tool's enrichment logic are automatically reflected in the lockfile the
// next time this tool is resolved or refreshed.
func lockedToolWithRenovate(lockRef string, version string, spec parsespec.Spec) modfile.LockedTool {
	lt := modfile.LockedTool{
		Ref:     lockRef,
		Version: version,
	}
	p, perr := Get(spec.Provider)
	if perr != nil {
		return lt
	}
	tt, terr := p.Tool(spec.Package)
	if terr != nil {
		return lt
	}

	// Create a skeleton of the actual structure that will live in the
	// lockfile's dependencies list and let the Tool mutate it directly.
	entry := modfile.RenovateDependency{
		Kind: "tool",
		Ref:  lockRef,
	}
	if entry.CurrentValue == "" {
		entry.CurrentValue = version
	}
	tt.EnrichLockfile(&entry)

	lt.DepName = entry.DepName
	lt.Datasource = entry.Datasource
	lt.PackageName = entry.PackageName
	lt.Versioning = entry.Versioning

	// The Tool had a chance to influence CurrentValue too.
	if entry.CurrentValue != "" {
		// We don't store CurrentValue on LockedTool (it lives on the dep),
		// but the upsert logic will pick it up via other paths if needed.
	}

	return lt
}
