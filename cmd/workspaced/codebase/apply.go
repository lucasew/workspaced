package codebase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"workspaced/pkg/cmdctx"
	"workspaced/pkg/configcue"
	"workspaced/pkg/deployer"
	"workspaced/pkg/dotfiles"
	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	"workspaced/pkg/source"
	"workspaced/pkg/taskgroup"
	"workspaced/pkg/tool"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		cmd := &cobra.Command{
			Use:   "apply [action]",
			Short: "Apply modules + templates to the repo root",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := cmd.Context()
				action := "switch"
				if len(args) > 0 {
					action = args[0]
				}
				_ = action // action is accepted for compatibility with home apply style

				dryRun := cmdctx.IsDryRun(ctx)
				showNoop, _ := cmd.Flags().GetBool("show-noop")

				g := taskgroup.MustFromContext(ctx)
				printReport := Schedule(g, cmd, dryRun, showNoop)
				runErr := taskgroup.Run(g)
				printReport()
				return runErr
			},
		}
		cmd.Flags().Bool("show-noop", false, "Also show files that would not change")
		parent.AddCommand(cmd)
	})
}

// Schedule wires codebase plan/apply.
// target is always the workspace root.
func Schedule(g *taskgroup.Group, cmd *cobra.Command, dryRun, showNoop bool) func() {
	taskName := "codebase:apply"
	updateMsg := "applying to repo root"
	if dryRun {
		taskName = "codebase:plan"
		updateMsg = "planning changes to repo root"
	}

	var finalResult *dotfiles.ApplyResult

	g.Go(taskName, taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update(updateMsg)
		s.Progress(0, 1)
		defer s.Progress(1, 1)

		logger := logging.GetLogger(ctx)

		// Discover the closest workspaced.cue from current CWD.
		// The directory containing it becomes the workspace root for
		// this "codebase" run (target + .workspaced/ location).
		// This lets sub-projects like LEWTEC/skills have their own
		// workspaced.cue (with their own modules/placers) and be treated
		// independently.
		cuePath, _ := configcue.ResolveWorkspaceCuePath(ctx, "")
		dotfilesRoot := ""
		if cuePath != "" {
			dotfilesRoot = filepath.Dir(cuePath)
		} else {
			// Fallback to git / dotfiles detection
			ws, err := modfile.DetectWorkspace(ctx, "")
			if err != nil {
				return fmt.Errorf("failed to detect workspace: %w", err)
			}
			dotfilesRoot = ws.Root
		}

		cfg, err := configcue.Load(ctx)  // will use the same discovery
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// For lockfile and modules base, we use the same root as the .cue
		ws := modfile.NewWorkspace(dotfilesRoot)
		lockResult, err := tool.RefreshWorkspaceLocks(ctx, ws, cfg)
		if err != nil {
			return fmt.Errorf("failed to refresh workspace lockfile: %w", err)
		}
		logger.Info("workspace lockfile refreshed", "sources", lockResult.Sources, "tools", lockResult.Tools)

		pipeline := source.NewPipeline()

		// Standard sources for codebase.
		// The workspace root is the dir containing the discovered workspaced.cue .
		configDir := filepath.Join(dotfilesRoot, ".workspaced", "config")
		modulesDir := filepath.Join(dotfilesRoot, "modules")

		stdOpts := source.StandardDotfilesOptions{
			ConfigTreeTarget: dotfilesRoot,
			RelocateTo:       dotfilesRoot,
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

		// State lives in the repo next to the lock
		// Repo-local state for codebase operations. Never use the global
		// ~/.config/workspaced state.
		statePath := filepath.Join(dotfilesRoot, ".workspaced", "state.json")
		stateStore, err := deployer.NewFileStateStore(statePath)
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

	return func() {
		printCodebasePlanOutput(finalResult, showNoop, dryRun)
	}
}

func printCodebasePlanOutput(result *dotfiles.ApplyResult, showNoop bool, dryRun bool) {
	if result == nil {
		return
	}
	if result.FilesCreated > 0 || result.FilesUpdated > 0 || result.FilesDeleted > 0 || (showNoop && result.FilesNoOp > 0) {
		orderedActions := deployer.SortActions(result.Actions)
		w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
		for _, a := range orderedActions {
			if a.Type == deployer.ActionNoop && !showNoop {
				continue
			}
			sourceInfo := ""
			if a.Desired.File != nil {
				sourceInfo = a.Desired.File.SourceInfo()
			}
			target := deployer.PrettyPath(a.Target)
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", a.Type, target, sourceInfo)
		}
		_ = w.Flush()

		fmt.Fprintf(os.Stderr, "\nSummary: %d created, %d updated, %d deleted", result.FilesCreated, result.FilesUpdated, result.FilesDeleted)
		if showNoop {
			fmt.Fprintf(os.Stderr, ", %d no-op", result.FilesNoOp)
		}
		fmt.Fprintln(os.Stderr)
	} else if !dryRun {
		fmt.Fprintln(os.Stderr, "No changes needed (repo root is up to date)")
	}
}


