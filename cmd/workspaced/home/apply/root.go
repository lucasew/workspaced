package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"workspaced/internal/apply"
	"workspaced/internal/cmdctx"
	"workspaced/internal/configcue"
	"workspaced/internal/deployer"
	"workspaced/internal/dotfiles"
	"workspaced/internal/modfile"
	_ "workspaced/internal/modfile/sourceprovider/prelude"
	"workspaced/internal/source"
	"workspaced/internal/tool"
	envdriver "workspaced/pkg/driver/env"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Declaratively apply system and user configurations",
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
	return cmd
}

// Schedule wires the home apply/plan work into the given task Group.
// Both "home apply" and "home plan" use this so the work always runs in-process
// under the caller's session. Register the returned func with Session.AfterWait
// so the plan/apply report prints after tasks finish and the UI/output env is gone.
func Schedule(g *taskgroup.Group, cmd *cobra.Command, dryRun, showNoop bool) func() error {
	taskName := "home:apply"
	updateMsg := "applying configuration"
	if dryRun {
		taskName = "home:plan"
		updateMsg = "planning changes"
	}

	logCtx := cmd.Context()
	var finalResult *dotfiles.ApplyResult

	g.Go(taskName, taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update(updateMsg)
		// Nested plan/apply Maps own aggregate bars; no Unit shell here.

		logger := logging.GetLogger(ctx)

		cfg, err := configcue.LoadHome(ctx)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		dotfilesRoot, err := envdriver.GetDotfilesRoot(ctx)
		if err != nil {
			return fmt.Errorf("failed to get dotfiles root: %w", err)
		}
		ws := modfile.NewWorkspace(dotfilesRoot)
		if _, err := tool.RefreshWorkspaceLocks(ctx, ws, cfg); err != nil {
			return fmt.Errorf("failed to refresh workspace lockfile: %w", err)
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		// 1. dconf marker plugin (home-specific)
		pipeline := source.NewPipeline()
		pipeline.AddPlugin(&apply.DconfPlugin{})

		// 2. Standard sources for this dotfiles repo targeting home
		configDir := filepath.Join(dotfilesRoot, "config")
		modulesDir := ws.ModulesBaseDir()

		stdOpts := source.StandardDotfilesOptions{
			ConfigTreeDir:    configDir,
			ConfigTreeTarget: home,
		}
		// Always provide ModulesDir even if it doesn't exist on disk yet.
		// This allows core:place (and other core modules) to be processed
		// without requiring a pre-existing modules/ directory.
		stdOpts.ModulesDir = modulesDir
		stdOpts.ModulesCfg = cfg

		stdPipeline, err := source.NewStandardDotfilesPipeline(ctx, cfg, stdOpts)
		if err != nil {
			return err
		}
		// Transfer plugins (dconf was added before, standard has the rest)
		for _, pl := range stdPipeline.GetPlugins() {
			pipeline.AddPlugin(pl)
		}

		// StateStore — paths on disk are relative to $HOME (~).
		stateStore, err := deployer.NewFileStateStore("~/.config/workspaced/state.json", home)
		if err != nil {
			return fmt.Errorf("failed to create state store: %w", err)
		}

		// Hooks
		hooks := []dotfiles.Hook{
			&dotfiles.FuncHook{
				AfterFn: func(ctx context.Context, actions []deployer.Action, execErr error) error {
					if execErr != nil {
						return nil
					}
					needsDconfApply := false
					for _, action := range actions {
						if action.Type != deployer.ActionCreate && action.Type != deployer.ActionUpdate {
							continue
						}
						// Match DconfPlugin, which places the marker under UserHomeDir
						// (not os.Getenv("HOME") — those can diverge when HOME is unset).
						if action.Desired.File != nil && deployer.GetTarget(action.Desired) == filepath.Join(home, ".config", "workspaced", "dconf.marker") {
							needsDconfApply = true
							break
						}
					}
					if !needsDconfApply {
						return nil
					}
					return apply.ApplyHomeDconf(ctx)
				},
			},
			// Hook to reload GTK theme
			&dotfiles.FuncHook{
				AfterFn: func(ctx context.Context, actions []deployer.Action, execErr error) error {
					if execErr != nil {
						return nil // Don't execute if there was an error
					}
					if envdriver.IsPhone(ctx) {
						return nil // Don't execute on phone
					}

					home, err := os.UserHomeDir()
					if err != nil {
						// Best-effort hook: skip GTK reload rather than fail apply.
						logger.Warn("failed to get home directory for gtk theme reload", "error", err)
						return nil
					}
					dummyTheme := filepath.Join(home, ".local", "share", "themes", "dummy")
					if _, err := os.Stat(dummyTheme); err == nil {
						targetTheme := "adw-gtk3-dark"
						if readCmd, err := execdriver.Run(ctx, "dconf", "read", "/org/gnome/desktop/interface/gtk-theme"); err == nil {
							if out, err := readCmd.Output(); err == nil {
								if v := strings.Trim(strings.TrimSpace(string(out)), "'"); v != "" {
									targetTheme = v
								}
							}
						}
						// Switch to dummy and back to force GTK reload
						if cmd, err := execdriver.Run(ctx, "dconf", "write", "/org/gnome/desktop/interface/gtk-theme", "'dummy'"); err == nil {
							if err := cmd.Run(); err != nil {
								logger.Warn("failed to switch to dummy theme", "error", err)
							}
						}
						if cmd, err := execdriver.Run(ctx, "dconf", "write", "/org/gnome/desktop/interface/gtk-theme", fmt.Sprintf("'%s'", targetTheme)); err == nil {
							if err := cmd.Run(); err != nil {
								logger.Warn("failed to restore gtk theme", "theme", targetTheme, "error", err)
							}
						}
					}
					return nil
				},
			},
		}

		mgr, err := dotfiles.NewManager(dotfiles.Config{
			Pipeline:   pipeline,
			StateStore: stateStore,
			Hooks:      hooks,
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
		dotfiles.LogApplyResult(logCtx, finalResult, dotfiles.LogApplyOptions{ShowNoop: showNoop})
		return nil
	}
}
