package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	cueerrors "cuelang.org/go/cue/errors"
	"workspaced/pkg/cmdctx"
	"workspaced/pkg/cmdregistry"
	"workspaced/pkg/configcue"
	envdriver "workspaced/pkg/driver/env"
	_ "workspaced/pkg/driver/prelude"
	"workspaced/pkg/logging"
	"workspaced/pkg/shellgen"
	"workspaced/pkg/taskgroup"
	_ "workspaced/pkg/tool/prelude"
	"workspaced/pkg/version"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func main() {
	// Bootstrap the process root context with a logger using the supported helper.
	// This eliminates direct context.Background for GetLogger paths.
	rootLogger := slog.New(logging.NewPlainHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	rootCtx := logging.NewRootContext(rootLogger)

	// This is the *only* place where a new logging context is created (attaching
	// the logger to a fresh tree from Background). All other uses of GetLogger
	// must be on a ctx passed down from here (or a child context derived from it).

	if os.Getenv("REBUILD_TEST") != "" {
		exe, err := os.Executable()
		if err != nil {
			panic(err)
		}
		h := sha256.New()
		f, err := os.Open(exe)
		if err != nil {
			panic(err)
		}
		defer logging.Close(rootCtx, f, "path", exe)
		if _, err = io.Copy(h, f); err != nil {
			panic(err)
		}
		logging.GetLogger(rootCtx).Info("build time", "t", h.Sum(nil))
	}
	// Load home config early to set driver weights.
	if _, err := configcue.LoadHome(rootCtx); err != nil {
		logging.GetLogger(rootCtx).Debug("failed to load config", "error", err)
	}

	var verbose bool
	var dryRun bool
	var cpuProfilePath string
	var memProfilePath string
	var stopProfiling func() error
	var rootSession *taskgroup.Session

	cmd := &cobra.Command{
		Use:           "workspaced",
		Short:         "workspaced - declarative user environment manager",
		Version:       version.GetBuildID(),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			// Ensure the command's context always carries a logger so that
			// GetLogger (and ReportError/Close/RunCleanup etc.) never see a
			// ctx without one. This is required now that GetLogger panics on
			// missing logger.
			cmdCtx := c.Context()
			if cmdCtx == nil {
				cmdCtx = rootCtx
			}
			if !logging.ContextHasLogger(cmdCtx) {
				cmdCtx = logging.ContextWithLogger(cmdCtx, rootLogger)
			}
			c.SetContext(cmdCtx)

			// Setup env paths using the connected root ctx (this is the place
			// where we call into env driver code that leads to GetLogger).
			// This ensures the ctx is the one from the actual root, not a
			// disconnected Background.
			envdriver.SetupEssentialPaths(c.Context())

			c.SetContext(cmdctx.WithVerbose(c.Context(), verbose))
			c.SetContext(cmdctx.WithDryRun(c.Context(), dryRun))

			// One session per invocation: root group + optional TUI observer.
			// Commands only schedule tasks; Close in PostRun waits and tears down.
			limits := taskgroup.DefaultLimits()
			if homeCfg, err := configcue.LoadHome(c.Context()); err == nil {
				limits = homeCfg.ConcurrencyLimits()
			}
			var sessCtx context.Context
			rootSession, sessCtx = taskgroup.Enter(c.Context(), limits)
			c.SetContext(sessCtx)

			if verbose {
				slog.SetLogLoggerLevel(slog.LevelDebug)
			}

			if stopProfiling != nil {
				return nil
			}
			cpuPath := cpuProfilePath
			if cpuPath == "" {
				cpuPath = os.Getenv("WORKSPACED_CPUPROFILE")
			}
			memPath := memProfilePath
			if memPath == "" {
				memPath = os.Getenv("WORKSPACED_MEMPROFILE")
			}

			var err error
			stopProfiling, err = startProfiling(c.Context(), cpuPath, memPath)
			if err == nil && (cpuPath != "" || memPath != "") {
				logger := logging.GetLogger(c.Context())
				logger.Info("profiling started", "cpu", cpuPath, "mem", memPath)
			}
			return err
		},
		PersistentPostRunE: func(c *cobra.Command, args []string) error {
			// Authoritative finish: Wait tasks, stop UI, restore stderr, AfterWait hooks.
			var sessErr error
			if rootSession != nil {
				sessErr = rootSession.Close()
				if sessErr != nil {
					logger := logging.GetLogger(c.Context())
					logger.Error("task group error", "err", sessErr)
				}
			}

			if stopProfiling == nil {
				return sessErr
			}
			err := stopProfiling()
			if err == nil && (cpuProfilePath != "" || memProfilePath != "" || os.Getenv("WORKSPACED_CPUPROFILE") != "" || os.Getenv("WORKSPACED_MEMPROFILE") != "") {
				logger := logging.GetLogger(c.Context())
				logger.Info("profiling finished")
			}
			stopProfiling = nil
			if sessErr != nil {
				return sessErr
			}
			return err
		},
	}

	// Global flags
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug logging")
	cmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "d", false, "Only show what would be done")
	cmd.PersistentFlags().StringVar(&cpuProfilePath, "cpuprofile", "", "Write CPU profile to file (or set WORKSPACED_CPUPROFILE)")
	cmd.PersistentFlags().StringVar(&memProfilePath, "memprofile", "", "Write heap profile to file at end (or set WORKSPACED_MEMPROFILE)")
	Registry.FillCommands(cmd)

	// Silence usage and errors on the entire command tree. Cobra by default
	// dumps full usage text whenever RunE returns an error (bad args, runtime
	// failures, etc). We want clean errors via the logger in main() only.
	setSilenceOnAllCommands(cmd)

	// Set root command for shell completion generation
	shellgen.SetRootCommand(cmd)

	if err := cmd.ExecuteContext(rootCtx); err != nil {
		if stopProfiling != nil {
			if stopErr := stopProfiling(); stopErr != nil {
				logger := logging.GetLogger(rootCtx)
				logger.Error("failed to stop profiling", "err", stopErr)
			}
		}
		logger := logging.GetLogger(rootCtx)
		if details := cueerrors.Details(err, nil); details != "" {
			logger.Error("error", "err", err, "details", "\n"+details)
		} else {
			logger.Error("error", "err", err)
		}
		os.Exit(1)
	}
}

func startProfiling(ctx context.Context, cpuProfilePath, memProfilePath string) (func() error, error) {
	var cpuFile *os.File
	profilingEnabled := cpuProfilePath != "" || memProfilePath != ""
	var minDurationWG sync.WaitGroup
	if profilingEnabled {
		minDurationWG.Add(1)
		go func() {
			defer minDurationWG.Done()
			time.Sleep(30 * time.Second)
		}()
	}

	if cpuProfilePath != "" {
		f, err := os.Create(cpuProfilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create cpuprofile file: %w", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			logging.Close(ctx, f, "path", cpuProfilePath)
			return nil, fmt.Errorf("failed to start CPU profile: %w", err)
		}
		cpuFile = f
	}

	return func() error {
		if profilingEnabled {
			minDurationWG.Wait()
		}
		if cpuFile != nil {
			pprof.StopCPUProfile()
			if err := cpuFile.Close(); err != nil {
				return err
			}
		}
		if memProfilePath != "" {
			f, err := os.Create(memProfilePath)
			if err != nil {
				return fmt.Errorf("failed to create memprofile file: %w", err)
			}
			runtime.GC()
			if err := pprof.WriteHeapProfile(f); err != nil {
				logging.Close(ctx, f, "path", memProfilePath)
				return fmt.Errorf("failed to write heap profile: %w", err)
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
		return nil
	}, nil
}

// setSilenceOnAllCommands disables Cobra's default behavior of printing
// usage text (and the error itself) when any command returns an error.
// We centralize error reporting in main() via the structured logger.
func setSilenceOnAllCommands(c *cobra.Command) {
	c.SilenceUsage = true
	c.SilenceErrors = true
	for _, sub := range c.Commands() {
		setSilenceOnAllCommands(sub)
	}
}
