package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	"workspaced/pkg/config"
	_ "workspaced/pkg/driver/prelude"
	"workspaced/pkg/registry"
	"workspaced/pkg/shellgen"
	"workspaced/pkg/version"

	"github.com/spf13/cobra"
)

var Registry registry.CommandRegistry

func main() {
	// Load config early to set driver weights
	if _, err := config.Load(); err != nil {
		slog.Debug("failed to load config", "error", err)
	}

	var verbose bool
	var cpuProfilePath string
	var memProfilePath string
	var stopProfiling func() error

	cmd := &cobra.Command{
		Use:     "workspaced",
		Short:   "workspaced - declarative user environment manager",
		Version: version.GetBuildID(),
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
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
			stopProfiling, err = startProfiling(cpuPath, memPath)
			if err == nil && (cpuPath != "" || memPath != "") {
				slog.Info("profiling started", "cpu", cpuPath, "mem", memPath)
			}
			return err
		},
		PersistentPostRunE: func(c *cobra.Command, args []string) error {
			if stopProfiling == nil {
				return nil
			}
			err := stopProfiling()
			if err == nil && (cpuProfilePath != "" || memProfilePath != "" || os.Getenv("WORKSPACED_CPUPROFILE") != "" || os.Getenv("WORKSPACED_MEMPROFILE") != "") {
				slog.Info("profiling finished")
			}
			stopProfiling = nil
			return err
		},
	}

	// Global flags
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug logging")
	cmd.PersistentFlags().StringVar(&cpuProfilePath, "cpuprofile", "", "Write CPU profile to file (or set WORKSPACED_CPUPROFILE)")
	cmd.PersistentFlags().StringVar(&memProfilePath, "memprofile", "", "Write heap profile to file at end (or set WORKSPACED_MEMPROFILE)")
	Registry.FillCommands(cmd)

	// Set root command for shell completion generation
	shellgen.SetRootCommand(cmd)

	if err := cmd.Execute(); err != nil {
		if stopProfiling != nil {
			if stopErr := stopProfiling(); stopErr != nil {
				slog.Error("failed to stop profiling", "err", stopErr)
			}
		}
		slog.Error("error", "err", err)
		os.Exit(1)
	}
}

func startProfiling(cpuProfilePath, memProfilePath string) (func() error, error) {
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
			_ = f.Close()
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
				_ = f.Close()
				return fmt.Errorf("failed to write heap profile: %w", err)
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
		return nil
	}, nil
}
