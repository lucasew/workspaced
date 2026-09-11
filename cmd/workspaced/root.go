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
	pkg_daemon "github.com/lucasew/workspaced/cmd/workspaced/daemon"
	"github.com/lucasew/workspaced/internal/clirun"
	"github.com/lucasew/workspaced/internal/cmdctx"
	"github.com/lucasew/workspaced/internal/configcue"
	"github.com/lucasew/workspaced/internal/shellgen"
	_ "github.com/lucasew/workspaced/internal/tool/prelude"
	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
	_ "github.com/lucasew/workspaced/pkg/driver/prelude"
	"github.com/lucasew/workspaced/pkg/logging"
	_ "github.com/lucasew/workspaced/pkg/palette/prelude"
	"github.com/lucasew/workspaced/pkg/taskgroup"

	"github.com/lewtec/lewkit/x/cmd"
)

func main() {
	rootLogger := slog.New(logging.NewPlainHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	rootCtx := logging.NewRootContext(rootLogger)

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
	if _, err := configcue.LoadHome(rootCtx); err != nil {
		logging.GetLogger(rootCtx).Debug("failed to load config", "error", err)
	}

	pkg_daemon.ExecuteCLI = executeCLI
	shellgen.SetRootSpec(Root{})

	if err := run(rootCtx); err != nil {
		logger := logging.GetLogger(rootCtx)
		if details := cueerrors.Details(err, nil); details != "" {
			logger.Error("error", "err", err, "details", "\n"+details)
		} else {
			logger.Error("error", "err", err)
		}
		os.Exit(1)
	}
}

func run(rootCtx context.Context) error {
	spec, err := cmd.Parse[Root](rewriteArgs(os.Args[1:])...)
	if err != nil {
		return err
	}
	if spec.Help.Value() || spec.Version.Value() {
		return spec.Run(rootCtx)
	}
	ctx, stop, session, err := setup(rootCtx, spec)
	if err != nil {
		return err
	}
	runErr := clirun.Run(ctx, &spec)
	var sessErr error
	if session != nil {
		sessErr = session.Close()
		if sessErr != nil {
			logging.GetLogger(ctx).Error("task group error", "err", sessErr)
		}
	}
	if stop != nil {
		if stopErr := stop(); stopErr != nil && runErr == nil && sessErr == nil {
			return stopErr
		}
	}
	if sessErr != nil {
		return sessErr
	}
	return runErr
}

func executeCLI(ctx context.Context, args []string) error {
	spec, err := cmd.Parse[Root](rewriteArgs(args)...)
	if err != nil {
		return err
	}
	return clirun.Run(ctx, &spec)
}

func setup(rootCtx context.Context, spec Root) (context.Context, func() error, *taskgroup.Session, error) {
	ctx := rootCtx
	if !logging.ContextHasLogger(ctx) {
		ctx = logging.ContextWithLogger(ctx, logging.GetLogger(rootCtx))
	}
	envdriver.SetupEssentialPaths(ctx)
	ctx = cmdctx.WithVerbose(ctx, spec.Verbose.Value())
	ctx = cmdctx.WithDryRun(ctx, spec.DryRun.Value())
	armedNoCache := spec.NoCache.Value() || cmdctx.EnvNoCache()
	ctx = cmdctx.WithNoCache(ctx, armedNoCache)
	if armedNoCache {
		logging.GetLogger(ctx).Info("no-cache enabled (flag or WORKSPACED_NO_CACHE)")
	}

	limits := taskgroup.DefaultLimits()
	if homeCfg, err := configcue.LoadHome(ctx); err == nil {
		limits = homeCfg.ConcurrencyLimits()
	}
	session, ctx := taskgroup.Enter(ctx, limits)

	if spec.Verbose.Value() {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	cpuPath := spec.CPUProfile.Value()
	if cpuPath == "" {
		cpuPath = os.Getenv("WORKSPACED_CPUPROFILE")
	}
	memPath := spec.MemProfile.Value()
	if memPath == "" {
		memPath = os.Getenv("WORKSPACED_MEMPROFILE")
	}
	stop, err := startProfiling(ctx, cpuPath, memPath)
	if err != nil {
		if closeErr := session.Close(); closeErr != nil {
			logging.GetLogger(ctx).Error("task group error", "err", closeErr)
		}
		return ctx, nil, nil, err
	}
	if cpuPath != "" || memPath != "" {
		logging.GetLogger(ctx).Info("profiling started", "cpu", cpuPath, "mem", memPath)
	}
	return ctx, func() error {
		err := stop()
		if err == nil && (cpuPath != "" || memPath != "") {
			logging.GetLogger(ctx).Info("profiling finished")
		}
		return err
	}, session, nil
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
			return nil, fmt.Errorf("create cpuprofile file: %w", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			logging.Close(ctx, f, "path", cpuProfilePath)
			return nil, fmt.Errorf("start CPU profile: %w", err)
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
				return fmt.Errorf("create memprofile file: %w", err)
			}
			runtime.GC()
			if err := pprof.WriteHeapProfile(f); err != nil {
				logging.Close(ctx, f, "path", memProfilePath)
				return fmt.Errorf("write heap profile: %w", err)
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
		return nil
	}, nil
}
