package codebase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"workspaced/pkg/cmdctx"
	"workspaced/pkg/configcue"
	"workspaced/pkg/deployer"
	"workspaced/pkg/dotfiles"
	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	_ "workspaced/pkg/modfile/sourceprovider/prelude"
	"workspaced/pkg/source"
	"workspaced/pkg/taskgroup"
	"workspaced/pkg/tool"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		cmd := &cobra.Command{
			Use:   "apply",
			Short: "Apply modules + templates to the repo root",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := cmd.Context()
				dryRun := cmdctx.IsDryRun(ctx)
				showNoop, _ := cmd.Flags().GetBool("show-noop")

				g := taskgroup.MustFromContext(ctx)
				printReport := Schedule(g, cmd, dryRun, showNoop)
				taskgroup.MustSessionFrom(ctx).AfterWait(printReport)
				return nil
			},
		}
		cmd.Flags().Bool("show-noop", false, "Also show files that would not change")
		parent.AddCommand(cmd)
	})
}

// Schedule wires codebase plan/apply.
// target is always the workspace root.
func Schedule(g *taskgroup.Group, cmd *cobra.Command, dryRun, showNoop bool) func() error {
	taskName := "codebase:apply"
	updateMsg := "applying to repo root"
	if dryRun {
		taskName = "codebase:plan"
		updateMsg = "planning changes to repo root"
	}

	logCtx := cmd.Context()
	var finalResult *dotfiles.ApplyResult

	g.Go(taskName, taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update(updateMsg)
		s.Progress(0, 1)

		defer s.Progress(1, 1)

		// Discover the closest workspaced.cue from current CWD (or fall back
		// to git root). The directory containing the cue is the workspace root
		// for this run: both the apply target and the lockfile location.
		//
		// This is deliberate. "codebase" is the general mechanism for operating
		// on *any* repo/tree that has a workspaced.cue (including sub-projects,
		// skill trees, random checkouts, the dotfiles repo itself, etc.).
		// It must not reach out to the user's personal dotfiles root.
		//
		// Locking uses the same *mechanism* as home apply:
		//   - Load config anchored to the specific workspace root
		//     (LoadForWorkspace, like home uses LoadHome for its root)
		//   - RefreshWorkspaceLocks (non-force path for sources + lazy tools)
		//     instead of the force=true mod lock path.
		// This makes ref/hash filling, skipping of already-locked HEAD inputs,
		// and tool lock enrichment behave consistently.
		cuePath, _ := configcue.ResolveWorkspaceCuePath(ctx, "")
		workspaceRoot := ""
		if cuePath != "" {
			workspaceRoot = filepath.Dir(cuePath)
		} else {
			// Fallback to git root (or dotfiles root as last resort)
			ws, err := modfile.DetectWorkspace(ctx, "")
			if err != nil {
				return fmt.Errorf("failed to detect workspace: %w", err)
			}
			workspaceRoot = ws.Root
		}

		cfg, err := configcue.LoadForWorkspace(ctx, workspaceRoot)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		ws := modfile.NewWorkspace(workspaceRoot)
		if _, err := tool.RefreshWorkspaceLocks(ctx, ws, cfg); err != nil {
			return fmt.Errorf("failed to refresh workspace lockfile: %w", err)
		}

		pipeline := source.NewPipeline()

		// Standard sources for codebase.
		// The workspace root is the dir containing the discovered workspaced.cue .
		configDir := filepath.Join(workspaceRoot, ".workspaced", "config")
		modulesDir := filepath.Join(workspaceRoot, "modules")

		stdOpts := source.StandardDotfilesOptions{
			ConfigTreeTarget: workspaceRoot,
			RelocateTo:       workspaceRoot,
		}
		if _, err := os.Stat(configDir); err == nil {
			stdOpts.ConfigTreeDir = configDir
		}
		// Always provide ModulesDir (even if not on disk) so placer modules
		// (core:place etc.) are considered even in fresh workspaces.
		stdOpts.ModulesDir = modulesDir
		stdOpts.ModulesCfg = cfg

		stdPipeline, err := source.NewStandardDotfilesPipeline(ctx, cfg, stdOpts)
		if err != nil {
			return err
		}
		for _, pl := range stdPipeline.GetPlugins() {
			pipeline.AddPlugin(pl)
		}

		// State lives in the repo next to the lock.
		// Repo-local state for codebase operations. Never use the global
		// ~/.config/workspaced state. Paths on disk are relative to workspace root.
		statePath := filepath.Join(workspaceRoot, ".workspaced", "state.json")
		stateStore, err := deployer.NewFileStateStore(statePath, workspaceRoot)
		if err != nil {
			return fmt.Errorf("failed to create state store: %w", err)
		}

		mgr, err := dotfiles.NewManager(dotfiles.Config{
			Pipeline:   pipeline,
			StateStore: stateStore,
			// No home-specific hooks (dconf, gtk, etc.)
		})
		if err != nil {
			return fmt.Errorf("failed to create manager: %w", err)
		}

		result, err := mgr.Apply(ctx, dotfiles.ApplyOptions{
			DryRun: dryRun,
		})
		if err != nil {
			return err
		}

		finalResult = result
		return nil
	})

	return func() error {
		logApplyResult(logCtx, finalResult, showNoop, dryRun)
		return nil
	}
}

func logApplyResult(ctx context.Context, result *dotfiles.ApplyResult, showNoop bool, dryRun bool) {
	if result == nil {
		return
	}
	logger := logging.GetLogger(ctx)
	hasChanges := result.FilesCreated > 0 || result.FilesUpdated > 0 || result.FilesDeleted > 0 || (showNoop && result.FilesNoOp > 0)
	if !hasChanges {
		if !dryRun {
			logger.Info("no changes needed", "target", "repo root")
		}
		return
	}
	for _, a := range deployer.SortActions(result.Actions) {
		if a.Type == deployer.ActionNoop && !showNoop {
			continue
		}
		sourceInfo := ""
		if a.Desired.File != nil {
			sourceInfo = a.Desired.File.SourceInfo()
		}
		logger.Info("apply action",
			"type", a.Type,
			"target", deployer.PrettyPath(a.Target),
			"source", sourceInfo,
		)
	}
	attrs := []any{
		"created", result.FilesCreated,
		"updated", result.FilesUpdated,
		"deleted", result.FilesDeleted,
	}
	if showNoop {
		attrs = append(attrs, "noop", result.FilesNoOp)
	}
	logger.Info("apply summary", attrs...)
}
