package dotfiles

import (
	"context"
	"errors"
	"fmt"
	"time"
	"workspaced/internal/deployer"
	"workspaced/internal/source"
	"workspaced/pkg/logging"
)

var (
	// ErrPipelineRequired is returned when a Manager is created without a pipeline.
	ErrPipelineRequired = errors.New("pipeline is required")
	// ErrStateStoreRequired is returned when a Manager is created without a state store.
	ErrStateStoreRequired = errors.New("state store is required")
)

// Manager is the main API for dotfiles management.
type Manager struct {
	pipeline   *source.Pipeline
	stateStore deployer.StateStore
	planner    *deployer.Planner
	executor   *deployer.Executor
	hooks      []Hook
}

// Config configures the Manager.
type Config struct {
	// Pipeline of plugins.
	Pipeline *source.Pipeline

	// StateStore for persistence.
	StateStore deployer.StateStore

	// Hooks (optional).
	Hooks []Hook
}

// NewManager creates a new Manager.
func NewManager(cfg Config) (*Manager, error) {
	if cfg.Pipeline == nil {
		return nil, ErrPipelineRequired
	}

	if cfg.StateStore == nil {
		return nil, ErrStateStoreRequired
	}

	return &Manager{
		pipeline:   cfg.Pipeline,
		stateStore: cfg.StateStore,
		planner:    deployer.NewPlanner(),
		executor:   deployer.NewExecutor(),
		hooks:      cfg.Hooks,
	}, nil
}

// ApplyOptions configures Apply execution.
type ApplyOptions struct {
	DryRun   bool // If true, only shows what would be done.
	ShowDiff bool // If true, shows a detailed diff.
}

// ApplyResult contains the result of Apply.
type ApplyResult struct {
	FilesCreated int
	FilesUpdated int
	FilesDeleted int
	FilesNoOp    int
	Actions      []deployer.Action
	Error        error
}

// Apply runs the full deployment cycle.
func (m *Manager) Apply(ctx context.Context, opts ApplyOptions) (*ApplyResult, error) {
	logger := logging.GetLogger(ctx)
	result := &ApplyResult{}

	// 1. Run pipeline
	logger.Info("running pipeline", "plugins", len(m.pipeline.GetPlugins()))
	files, err := m.pipeline.Run(ctx, []source.File{})
	if err != nil {
		result.Error = err
		return result, fmt.Errorf("failed to run pipeline: %w", err)
	}

	logger.Info("pipeline completed", "files", len(files))

	// 2. Convert source.File to deployer.DesiredState
	desired := make([]deployer.DesiredState, len(files))
	for i, f := range files {
		desired[i] = deployer.DesiredState{
			File: f,
		}
	}

	// 3. Load current state
	logger.Info("loading state", "store", m.stateStore.Path())
	state, err := m.stateStore.Load()
	if err != nil {
		result.Error = err
		return result, fmt.Errorf("failed to load state: %w", err)
	}

	// 4. Plan actions
	logger.Info("planning actions")
	planStart := time.Now()
	actions, err := m.planner.Plan(ctx, desired, state)
	if err != nil {
		result.Error = err
		return result, fmt.Errorf("failed to plan: %w", err)
	}
	logger.Info("plan calculated", "duration", time.Since(planStart).String(), "actions", len(actions))

	result.Actions = actions

	// Count actions
	for _, a := range actions {
		switch a.Type {
		case deployer.ActionCreate:
			result.FilesCreated++
		case deployer.ActionUpdate:
			result.FilesUpdated++
		case deployer.ActionDelete:
			result.FilesDeleted++
		case deployer.ActionNoop:
			result.FilesNoOp++
		}
	}

	hasChanges := result.FilesCreated > 0 || result.FilesUpdated > 0 || result.FilesDeleted > 0

	if !hasChanges {
		logger.Info("no changes needed")
		return result, nil
	}

	logger.Info("changes planned",
		"create", result.FilesCreated,
		"update", result.FilesUpdated,
		"delete", result.FilesDeleted,
	)

	// Dry-run: stop here
	if opts.DryRun {
		logger.Info("dry-run: skipping execution")
		return result, nil
	}

	// 5. Execute Before hooks
	for _, hook := range m.hooks {
		if err := hook.Before(ctx, actions); err != nil {
			result.Error = err
			return result, fmt.Errorf("hook before failed: %w", err)
		}
	}

	// 6. Execute actions
	logger.Info("executing actions")
	execErr := m.executor.Execute(ctx, actions, state)

	// 7. Execute After hooks (even if there was an error)
	for _, hook := range m.hooks {
		if err := hook.After(ctx, actions, execErr); err != nil {
			logger.Error("hook after failed", "error", err)
			// Continue executing other hooks
		}
	}

	if execErr != nil {
		result.Error = execErr
		return result, fmt.Errorf("failed to execute: %w", execErr)
	}

	// 8. Save state
	logger.Info("saving state")
	if err := m.stateStore.Save(state); err != nil {
		result.Error = err
		return result, fmt.Errorf("failed to save state: %w", err)
	}

	logger.Info("apply completed successfully")
	return result, nil
}

// GetPipeline returns the configured pipeline.
func (m *Manager) GetPipeline() *source.Pipeline {
	return m.pipeline
}

// GetStateStore returns the configured state store.
func (m *Manager) GetStateStore() deployer.StateStore {
	return m.stateStore
}
