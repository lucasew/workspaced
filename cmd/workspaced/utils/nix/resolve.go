package nix

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/workspaced/internal/executil"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
)

// parseFlakeRef splits "repo#item" or "repo#item/binary" into repo, item, binary.
func parseFlakeRef(ref string) (repo, item, binary string) {
	parts := strings.Split(ref, "#")
	repo = parts[0]
	if len(parts) > 1 {
		item = parts[1]
	}
	if strings.Contains(item, "/") {
		itemParts := strings.Split(item, "/")
		item = itemParts[0]
		binary = itemParts[1]
	}
	return repo, item, binary
}

// runFromResultPath finds a binary under resultPath/bin and runs it with runArgs.
func runFromResultPath(ctx context.Context, resultPath, binary string, runArgs []string) error {
	binDir := filepath.Join(resultPath, "bin")
	if binary == "" {
		entries, err := os.ReadDir(binDir)
		if err != nil || len(entries) == 0 {
			return fmt.Errorf("%w: %s", ErrNoBinaryFound, binDir)
		}
		binary = entries[0].Name()
	}

	binPath := filepath.Join(binDir, binary)
	if _, err := os.Stat(binPath); err != nil {
		entries, rerr := os.ReadDir(binDir)
		if rerr != nil {
			return rerr
		}
		for _, entry := range entries {
			if strings.Contains(entry.Name(), binary) {
				binPath = filepath.Join(binDir, entry.Name())
				break
			}
		}
	}

	ec := execdriver.MustRun(ctx, binPath, runArgs...)
	executil.InheritContextWriters(ctx, ec)
	ec.Stdin = os.Stdin
	return ec.Run()
}

// stripLeadingDashArgs drops a leading "--" from args (cobra DisableFlagParsing).
func stripLeadingDashArgs(args []string) []string {
	if len(args) > 0 && args[0] == "--" {
		return args[1:]
	}
	return args
}

// buildFlakeFn builds a flake ref (repo#item) and returns the store result path.
type buildFlakeFn func(ctx context.Context, flakeRef string) (resultPath string, err error)

// runFlakeRef parses args[0] as a flake ref, builds it, and runs the binary with remaining args.
func runFlakeRef(ctx context.Context, args []string, build buildFlakeFn) error {
	if len(args) == 0 {
		return ErrNoFlakeRef
	}
	ref := args[0]
	runArgs := stripLeadingDashArgs(args[1:])
	repo, item, binary := parseFlakeRef(ref)
	resultPath, err := build(ctx, repo+"#"+item)
	if err != nil {
		return err
	}
	return runFromResultPath(ctx, resultPath, binary, runArgs)
}
