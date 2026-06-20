package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"workspaced/pkg/apply"
	"workspaced/pkg/cmdctx"
	"workspaced/pkg/configcue"
	"workspaced/pkg/deployer"
	"workspaced/pkg/dotfiles"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/env"
	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	_ "workspaced/pkg/modfile/sourceprovider/prelude"
	"workspaced/pkg/source"
	"workspaced/pkg/taskgroup"
	"workspaced/pkg/template"
	"workspaced/pkg/tool"

	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply [action]",
		Short: "Declaratively apply system and user configurations",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			action := "switch"
			if len(args) > 0 {
				action = args[0]
			}
			_ = action

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
	return cmd
}

// Schedule wires the home apply/plan work into the given task Group.
// Both "home apply" and "home plan" use this so the work always runs in-process
// (under the caller's taskgroup and any active bubbletea renderer).
// The caller is responsible for calling taskgroup.Run(g) afterwards and then
// calling the returned function to emit the final plan/apply report (after any
// bubbletea renderer has exited, so direct stderr writes are reliable).
func Schedule(g *taskgroup.Group, cmd *cobra.Command, dryRun, showNoop bool) func() {
	taskName := "home:apply"
	updateMsg := "applying configuration"
	if dryRun {
		taskName = "home:plan"
		updateMsg = "planning changes"
	}

	var finalResult *dotfiles.ApplyResult

	g.Go(taskName, taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update(updateMsg)
		s.Progress(0, 1)
		defer s.Progress(1, 1)

		logger := logging.GetLogger(ctx)

		cfg, err := configcue.LoadHome(ctx)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		dotfilesRoot, err := env.GetDotfilesRoot(ctx)
		if err != nil {
			return fmt.Errorf("failed to get dotfiles root: %w", err)
		}
		ws := modfile.NewWorkspace(dotfilesRoot)
		lockResult, err := tool.RefreshWorkspaceLocks(ctx, ws, cfg)
		if err != nil {
			return fmt.Errorf("failed to refresh workspace lockfile: %w", err)
		}
		logger.Info("workspace lockfile refreshed", "sources", lockResult.Sources, "tools", lockResult.Tools)

		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		engine := template.NewEngine(ctx)

		// Configure plugin pipeline
		configDir := filepath.Join(dotfilesRoot, "config")
		pipeline := source.NewPipeline()

		// 1. Provider dconf (legacy)
		pipeline.AddPlugin(source.NewProviderPlugin(&apply.DconfProvider{}, 50))

		// 2. Scanner - discovers files in config/
		if _, err := os.Stat(configDir); err == nil {
			scanner, err := source.NewScannerPlugin(source.ScannerConfig{
				Name:       "legacy-config",
				BaseDir:    configDir,
				TargetBase: home,
				Priority:   50, // Legacy has lower priority than modules
			})
			if err != nil {
				return fmt.Errorf("failed to create scanner: %w", err)
			}
			pipeline.AddPlugin(scanner)
		}

		// 2.5 Modules Scanner
		modulesDir := ws.ModulesBaseDir()
		if _, err := os.Stat(modulesDir); err == nil {
			pipeline.AddPlugin(source.NewModuleScannerPlugin(modulesDir, cfg, 100))
		}

		// 3. TemplateExpander - renders .tmpl (includes multi-file)
		pipeline.AddPlugin(source.NewTemplateExpanderPlugin(engine, cfg))

		// 4. DotDProcessor - concatenates .d.tmpl/
		pipeline.AddPlugin(source.NewDotDProcessorPlugin(engine, cfg))

		// 5. StrictConflictResolver - ensures total uniqueness
		pipeline.AddPlugin(source.NewStrictConflictResolverPlugin())

		// StateStore
		stateStore, err := deployer.NewFileStateStore("~/.config/workspaced/state.json")
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
						if action.Desired.File != nil && deployer.GetTarget(action.Desired) == filepath.Join(os.Getenv("HOME"), ".config", "workspaced", "dconf.marker") {
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
					if env.IsPhone(ctx) {
						return nil // Don't execute on phone
					}

					home, _ := os.UserHomeDir()
					dummyTheme := home + "/.local/share/themes/dummy"
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

	return func() {
		printPlanOutput(finalResult, showNoop)
	}
}

func printPlanOutput(result *dotfiles.ApplyResult, showNoop bool) {
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
	}
}
