package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"workspaced/pkg/apply"
	"workspaced/pkg/config"
	"workspaced/pkg/deployer"
	"workspaced/pkg/dotfiles"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/env"
	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	_ "workspaced/pkg/modfile/sourceprovider/prelude"
	"workspaced/pkg/source"
	"workspaced/pkg/template"

	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply [action]",
		Short: "Declaratively apply system and user configurations",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			logger := logging.GetLogger(ctx)

			action := "switch"
			if len(args) > 0 {
				action = args[0]
			}
			_ = action

			dryRun, _ := cmd.Flags().GetBool("dry-run")

			// Carregar configuração
			cfg, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Obter dotfiles root
			dotfilesRoot, err := env.GetDotfilesRoot()
			if err != nil {
				return fmt.Errorf("failed to get dotfiles root: %w", err)
			}
			ws := modfile.NewWorkspace(dotfilesRoot)
			lockResult, err := modfile.GenerateLock(ctx, ws)
			if err != nil {
				return fmt.Errorf("failed to refresh module lockfile: %w", err)
			}
			logger.Info("module lockfile refreshed", "sources", lockResult.Sources, "modules", lockResult.Modules)

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}

			// Criar template engine compartilhada
			engine := template.NewEngine(ctx)

			// Configurar pipeline de plugins
			configDir := filepath.Join(dotfilesRoot, "config")
			pipeline := source.NewPipeline()

			// 1. Provider dconf (legacy)
			pipeline.AddPlugin(source.NewProviderPlugin(&apply.DconfProvider{}, 50))

			// 2. Scanner - descobre arquivos em config/
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

			// 3. TemplateExpander - renderiza .tmpl (inclui multi-file)
			pipeline.AddPlugin(source.NewTemplateExpanderPlugin(engine, cfg))

			// 4. DotDProcessor - concatena .d.tmpl/
			pipeline.AddPlugin(source.NewDotDProcessorPlugin(engine, cfg))

			// 5. StrictConflictResolver - garante unicidade total
			pipeline.AddPlugin(source.NewStrictConflictResolverPlugin())

			// StateStore
			stateStore, err := deployer.NewFileStateStore("~/.config/workspaced/state.json")
			if err != nil {
				return fmt.Errorf("failed to create state store: %w", err)
			}

			// Hooks
			hooks := []dotfiles.Hook{
				// Hook para reload GTK theme
				&dotfiles.FuncHook{
					AfterFn: func(ctx context.Context, actions []deployer.Action, execErr error) error {
						if execErr != nil {
							return nil // Não executar se houve erro
						}
						if env.IsPhone() {
							return nil // Não executar em phone
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

			// Criar manager com pipeline
			mgr, err := dotfiles.NewManager(dotfiles.Config{
				Pipeline:   pipeline,
				StateStore: stateStore,
				Hooks:      hooks,
			})
			if err != nil {
				return fmt.Errorf("failed to create manager: %w", err)
			}

			// Aplicar configurações
			result, err := mgr.Apply(ctx, dotfiles.ApplyOptions{
				DryRun: dryRun,
			})
			if err != nil {
				return err
			}

			// Mostrar resultado
			if result.FilesCreated > 0 || result.FilesUpdated > 0 || result.FilesDeleted > 0 {
				for _, a := range result.Actions {
					if a.Type != deployer.ActionNoop {
						cmd.Printf("[%s] %s\n", a.Type, a.Target)
						if a.Type == deployer.ActionUpdate || a.Type == deployer.ActionCreate {
							cmd.Printf("      -> %s\n", a.Desired.File.SourceInfo())
						}
					}
				}
				cmd.Printf("\nSummary: %d created, %d updated, %d deleted\n", result.FilesCreated, result.FilesUpdated, result.FilesDeleted)
			}

			return nil
		},
	}
	cmd.Flags().BoolP("dry-run", "d", false, "Only show what would be done")
	return cmd
}
