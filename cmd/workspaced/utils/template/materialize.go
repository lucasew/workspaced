package template

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/configcue"
	"github.com/lucasew/workspaced/internal/deployer"
	"github.com/lucasew/workspaced/internal/source"
)

var ErrTargetRequired = errors.New("--target is required")

type Materialize struct {
	Config []cmd.StringArg `short:"c" long:"config" help:"Configuration file(s) to merge"`
	Source []cmd.StringArg `short:"s" long:"source" help:"Source directory(ies) to scan"`
	Target cmd.StringArg   `short:"t" long:"target" help:"Target directory to materialize files into"`
}

func (Materialize) Description() string { return "Materialize templates into a directory (low-level)" }

func (m *Materialize) Run(ctx context.Context) error {
	targetDir := m.Target.Value()
	if targetDir == "" {
		return ErrTargetRequired
	}
	cfg, err := configcue.LoadFiles(ctx, cmd.Values(m.Config))
	if err != nil {
		return err
	}
	sourcePaths := cmd.Values(m.Source)
	providers := make([]source.Plugin, 0, len(sourcePaths))
	for i, srcPath := range sourcePaths {
		absSrc, err := filepath.Abs(srcPath)
		if err != nil {
			return err
		}
		scanner, err := source.NewScannerPlugin(source.ScannerConfig{Name: fmt.Sprintf("source-%d", i), BaseDir: absSrc, TargetBase: targetDir, Priority: 100})
		if err != nil {
			return err
		}
		providers = append(providers, scanner)
	}
	tree, err := source.Builder{Config: cfg, TargetBase: targetDir, Providers: providers}.Tree(ctx)
	if err != nil {
		return err
	}
	executor := deployer.NewExecutor()
	actions := []deployer.Action{}
	for _, f := range tree.Files() {
		actions = append(actions, deployer.Action{Type: deployer.ActionCreate, Target: filepath.Join(f.TargetBase(), f.RelPath()), Desired: deployer.DesiredState{File: f}})
	}
	return executor.Execute(ctx, actions, &deployer.State{Files: make(map[string]deployer.ManagedInfo)})
}
