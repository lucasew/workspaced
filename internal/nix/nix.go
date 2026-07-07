package nix

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"workspaced/pkg/api"
	envdriver "workspaced/pkg/driver/env"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/notification"
	"workspaced/internal/executil"
	"workspaced/internal/icons"
	"workspaced/pkg/logging"
	"workspaced/internal/sudo"
	"workspaced/internal/types"
)

var buildCache sync.Map // key: sourcePath#attribute, value: resultPath

type Direction int

const (
	To Direction = iota
	From
)

func parseFlakeRef(ref string) (repo string, item string) {
	parts := strings.SplitN(ref, "#", 2)
	repo = parts[0]
	if len(parts) > 1 {
		item = parts[1]
	}
	return
}

// nixCmd builds a command with explicit stdout/stderr (context writers or
// process streams). Never leaves either stream nil.
func nixCmd(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) *exec.Cmd {
	cmd := execdriver.MustRun(ctx, name, args...)
	if stdout == nil {
		stdout = executil.StdoutOr(ctx, os.Stdout)
	}
	if stderr == nil {
		stderr = executil.StderrOr(ctx, os.Stderr)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd
}

// nixOutput runs name/args with stderr on the process/context stream and
// captures stdout (e.g. nix build --print-out-paths, --json).
func nixOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	var stdout bytes.Buffer
	cmd := nixCmd(ctx, &stdout, nil, name, args...)
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

// nixRun streams both stdout and stderr to context/process writers.
func nixRun(ctx context.Context, name string, args ...string) error {
	return nixCmd(ctx, nil, nil, name, args...).Run()
}

func ResolveFlakePath(ctx context.Context, repo string) (string, error) {
	if repo == "" || repo == "." || repo == "," {
		root, err := envdriver.GetDotfilesRoot(ctx)
		if err != nil {
			return "", err
		}
		repo = root
	}

	out, err := nixOutput(ctx, "nix", "flake", "archive", repo, "--json")
	if err != nil {
		return "", fmt.Errorf("failed to archive flake %s to store: %w", repo, err)
	}

	var meta struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(out, &meta); err != nil {
		return "", fmt.Errorf("failed to parse flake archive output: %w", err)
	}

	return meta.Path, nil
}

func CopyClosure(ctx context.Context, target string, path string, direction Direction) error {
	args := []string{}
	if direction == To {
		args = append(args, "-s", "--to", target, path)
	} else {
		args = append(args, "--from", target, path)
	}
	if err := nixRun(ctx, "nix-copy-closure", args...); err != nil {
		return err
	}
	return nil
}

func GetRemoteCacheDir(ctx context.Context, target string) (string, error) {
	script := `echo "${XDG_RUNTIME_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}}/rbuild-outputs"`
	out, err := nixOutput(ctx, "ssh", target, script)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func RemoteBuild(ctx context.Context, ref string, target string, copyBack bool) (string, error) {
	logger := logging.GetLogger(ctx)

	if target == "" {
		target = os.Getenv("NIX_RBUILD_TARGET")
		if target == "" {
			target = "whiterun"
		}
	}

	n := &notification.Notification{
		ID:          notification.NixBuildNotificationID,
		Title:       "Nix Remote Build",
		HasProgress: true,
	}
	if icon, err := icons.GetIconPath(ctx, "https://nixos.org"); err == nil {
		n.Icon = icon
	}

	updateProgress := func(msg string, prog float64) {
		n.Message = msg
		n.Progress = prog
		logging.ReportError(ctx, notification.Notify(ctx, n))
		logger.Info(msg, "progress", prog)
	}

	updateProgress("Resolving flake metadata...", 0.1)
	repo, item := parseFlakeRef(ref)

	sourcePath, err := ResolveFlakePath(ctx, repo)
	if err != nil {
		return "", err
	}

	updateProgress(fmt.Sprintf("Syncing sources to %s...", target), 0.3)
	if err := CopyClosure(ctx, target, sourcePath, To); err != nil {
		return "", fmt.Errorf("failed to copy source to %s: %w", target, err)
	}

	updateProgress("Building on remote server...", 0.6)
	remoteCache, err := GetRemoteCacheDir(ctx, target)
	if err != nil {
		return "", fmt.Errorf("failed to get remote cache dir: %w", err)
	}

	buildID := make([]byte, 8)
	_, _ = rand.Read(buildID)
	uuid := fmt.Sprintf("%x", buildID)
	outLink := fmt.Sprintf("%s/%s", remoteCache, uuid)

	safeRef := fmt.Sprintf("%s#%s", sourcePath, item)
	remoteArgs := []string{
		target, "-t",
		"mkdir", "-p", remoteCache, "&&",
		"nix", "build", "-L", fmt.Sprintf("%q", safeRef), "--out-link", outLink, "--show-trace",
	}
	if err := nixRun(ctx, "ssh", remoteArgs...); err != nil {
		return "", fmt.Errorf("%w: remote build failed: %w", api.ErrBuildFailed, err)
	}

	out, err := nixOutput(ctx, "ssh", target, "realpath", outLink)
	if err != nil {
		return "", fmt.Errorf("failed to resolve result path: %w", err)
	}
	resultPath := strings.TrimSpace(string(out))

	if copyBack {
		updateProgress("Syncing result back...", 0.9)
		if err := CopyClosure(ctx, target, resultPath, From); err != nil {
			return "", fmt.Errorf("failed to copy result from %s: %w", target, err)
		}
	}

	updateProgress("Build completed successfully.", 1.0)
	return resultPath, nil
}

func Build(ctx context.Context, ref string, useCache bool) (string, error) {
	logger := logging.GetLogger(ctx)

	repo, item := parseFlakeRef(ref)

	sourcePath, err := ResolveFlakePath(ctx, repo)
	if err != nil {
		return "", err
	}

	cacheKey := fmt.Sprintf("%s#%s", sourcePath, item)
	if useCache {
		if val, ok := buildCache.Load(cacheKey); ok {
			resultPath := val.(string)
			if _, err := os.Stat(resultPath); err == nil {
				logger.Debug("build cache hit", "ref", ref, "path", resultPath)
				return resultPath, nil
			}
			buildCache.Delete(cacheKey)
		}
	}

	logger.Info("performing nix build", "ref", ref)
	// -L streams build logs on stderr; stdout stays --print-out-paths only.
	out, err := nixOutput(ctx, "nix", "build", "-L", fmt.Sprintf("%s#%s", sourcePath, item), "--no-link", "--print-out-paths")
	if err != nil {
		return "", fmt.Errorf("%w: nix build failed: %w", api.ErrBuildFailed, err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	resultPath := lines[0]
	for _, line := range lines {
		if info, err := os.Stat(filepath.Join(line, "bin")); err == nil && info.IsDir() {
			resultPath = line
			break
		}
	}

	if useCache {
		buildCache.Store(cacheKey, resultPath)
	}

	return resultPath, nil
}

func Rebuild(ctx context.Context, action string, flake string) error {
	hostname, err := envdriver.GetHostname(ctx)
	if err != nil {
		return fmt.Errorf("hostname: %w", err)
	}
	if flake == "" || flake == "." || flake == "," {
		root, err := envdriver.GetDotfilesRoot(ctx)
		if err != nil {
			return err
		}
		flake = root
	}

	if envdriver.IsInStore(ctx) {
		flake = "github:lucasew/nixcfg"
	}

	supportedNodes := []string{"riverwood", "whiterun", "ravenrock", "atomicpi", "recovery"}
	if !slices.Contains(supportedNodes, hostname) {
		return fmt.Errorf("%w: %s", api.ErrHostNotFound, hostname)
	}

	var toplevel string
	if strings.HasPrefix(flake, "/nix/store/") {
		toplevel = flake
	} else {
		ref := fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.toplevel", flake, hostname)
		var err error
		toplevel, err = Build(ctx, ref, true)
		if err != nil {
			return err
		}
	}

	cmdName := filepath.Join(toplevel, "bin/switch-to-configuration")
	args := []string{action}

	if os.Getuid() != 0 {
		return sudo.Enqueue(ctx, &types.SudoCommand{
			Slug:    "rebuild",
			Command: cmdName,
			Args:    args,
		})
	}
	return nixRun(ctx, cmdName, args...)
}

func HomeManagerSwitch(ctx context.Context, action string, flake string) error {
	if flake == "" || flake == "." || flake == "," {
		root, err := envdriver.GetDotfilesRoot(ctx)
		if err != nil {
			return err
		}
		flake = root
	}

	if envdriver.IsInStore(ctx) {
		flake = "github:lucasew/nixcfg"
	}

	var activationPackage string
	if strings.HasPrefix(flake, "/nix/store/") {
		activationPackage = flake
	} else {
		ref := fmt.Sprintf("%s#homeConfigurations.main.activationPackage", flake)
		var err error
		activationPackage, err = Build(ctx, ref, true)
		if err != nil {
			return err
		}
	}

	activatePath := filepath.Join(activationPackage, "activate")
	if _, err := os.Stat(activatePath); err != nil {
		return fmt.Errorf("activation script not found at %s: %w", activatePath, err)
	}

	return nixRun(ctx, activatePath)
}

func GetFlakeOutput(ctx context.Context, flake, output string) (string, error) {
	out, err := nixOutput(ctx, "nix", "build", "-L", fmt.Sprintf("%s#%s", flake, output), "--no-link", "--print-out-paths")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
