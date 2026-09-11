package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lewtec/lewkit/x/cmd"
	pkg_daemon "github.com/lucasew/workspaced/cmd/workspaced/daemon"
	"github.com/lucasew/workspaced/internal/clirun"
	"github.com/lucasew/workspaced/internal/version"
)

// Root is the workspaced CLI spec.
type Root struct {
	Verbose    cmd.Flag      `short:"v" long:"verbose" help:"Enable debug logging"`
	DryRun     cmd.Flag      `short:"d" long:"dry-run" help:"Only show what would be done"`
	NoCache    cmd.Flag      `long:"no-cache" help:"Ignore install/module/source/shell caches; re-fetch locked tools; treat deploy noops as updates (also WORKSPACED_NO_CACHE)"`
	CPUProfile cmd.StringArg `long:"cpuprofile" help:"Write CPU profile to file (or set WORKSPACED_CPUPROFILE)"`
	MemProfile cmd.StringArg `long:"memprofile" help:"Write heap profile to file at end (or set WORKSPACED_MEMPROFILE)"`
	Help       cmd.Flag      `short:"h" long:"help" help:"show help"`
	Version    cmd.Flag      `long:"version" help:"print version"`

	children `flatten:""`
	Daemon   *pkg_daemon.Command
}

func (Root) Description() string {
	return "workspaced - declarative user environment manager"
}

func (r *Root) Run(ctx context.Context) error {
	if r.Help.Value() {
		return clirun.PrintUsage[Root]("workspaced")
	}
	if r.Version.Value() {
		_, err := fmt.Fprintln(os.Stdout, version.VersionString())
		return err
	}
	return clirun.PrintUsage[Root]("workspaced")
}

func rewriteArgs(args []string) []string {
	return rewriteOpen(args)
}

func rewriteOpen(args []string) []string {
	i := commandIndex(args, "open")
	if i < 0 || i+1 >= len(args) {
		return args
	}
	next := args[i+1]
	if isOption(next) || isOpenSub(next) {
		return args
	}
	out := make([]string, 0, len(args)+1)
	out = append(out, args[:i+1]...)
	out = append(out, "file")
	out = append(out, args[i+1:]...)
	return out
}

func isOpenSub(s string) bool {
	switch s {
	case "file", "webapp", "terminal", "exec", "lazy", "mise":
		return true
	default:
		return false
	}
}

func commandIndex(args []string, name string) int {
	skipVal := false
	for i, a := range args {
		if skipVal {
			skipVal = false
			continue
		}
		if a == "--" {
			return -1
		}
		if isOption(a) {
			if optionTakesValue(a) {
				skipVal = !strings.Contains(a, "=")
			}
			continue
		}
		if a == name {
			return i
		}
		// first non-flag token that isn't open means a different command
		return -1
	}
	return -1
}

func isOption(a string) bool {
	return len(a) > 1 && a[0] == '-' && a != "-"
}

func optionTakesValue(a string) bool {
	name := strings.TrimLeft(a, "-")
	name, _, _ = strings.Cut(name, "=")
	switch name {
	case "cpuprofile", "memprofile":
		return true
	default:
		return false
	}
}
