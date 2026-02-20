package template

import (
	"fmt"
	"path/filepath"
	"workspaced/pkg/config"
	"workspaced/pkg/deployer"
	"workspaced/pkg/source"
	"workspaced/pkg/template"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(
		func(c *cobra.Command) {
			var configPaths []string
			var sourcePaths []string
			var targetDir string
			cmd := &cobra.Command{Use: "materialize", Short: "Materialize templates into a directory (low-level)", RunE: func(c *cobra.Command, args []string) error {
				ctx := c.Context()
				if targetDir == "" {
					return fmt.Errorf("--target is required")
				}
				cfg, err := config.LoadFiles(configPaths)
				if err != nil {
					return err
				}
				engine := template.NewEngine(ctx)
				pipeline := source.NewPipeline()
				for i, srcPath := range sourcePaths {
					absSrc, err := filepath.Abs(srcPath)
					if err != nil {
						return err
					}
					scanner, err := source.NewScannerPlugin(source.ScannerConfig{Name: fmt.Sprintf("source-%d", i), BaseDir: absSrc, TargetBase: targetDir, Priority: 100})
					if err != nil {
						return err
					}
					pipeline.AddPlugin(scanner)
				}
				pipeline.AddPlugin(source.NewTemplateExpanderPlugin(engine, cfg))
				pipeline.AddPlugin(source.NewDotDProcessorPlugin(engine, cfg))
				pipeline.AddPlugin(source.NewStrictConflictResolverPlugin())
				files, err := pipeline.Run(ctx, nil)
				if err != nil {
					return err
				}
				executor := deployer.NewExecutor()
				actions := []deployer.Action{}
				for _, f := range files {
					actions = append(actions, deployer.Action{Type: deployer.ActionCreate, Target: filepath.Join(f.TargetBase(), f.RelPath()), Desired: deployer.DesiredState{File: f}})
				}
				return executor.Execute(ctx, actions, &deployer.State{Files: make(map[string]deployer.ManagedInfo)})
			}}
			cmd.Flags().StringSliceVarP(&configPaths, "config", "c", nil, "Configuration file(s) to merge")
			cmd.Flags().StringSliceVarP(&sourcePaths, "source", "s", nil, "Source directory(ies) to scan")
			cmd.Flags().StringVarP(&targetDir, "target", "t", "", "Target directory to materialize files into")
			c.AddCommand(cmd)

		})
}
