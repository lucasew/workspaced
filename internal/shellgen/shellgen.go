package shellgen

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

// Generator is a function that generates shell code
type Generator func(context.Context) (string, error)

// rootCommand is set by SetRootCommand and used by generators that need it
var rootCommand *cobra.Command

// SetRootCommand sets the root command for generators that need it (e.g., completion)
func SetRootCommand(cmd *cobra.Command) {
	rootCommand = cmd
}

// generators maps order/name to generator functions
var generators = map[string]Generator{
	"05-flags":      func(ctx context.Context) (string, error) { return GenerateFlags(ctx) },
	"06-daemon":     func(ctx context.Context) (string, error) { return GenerateDaemon() },
	"10-completion": func(ctx context.Context) (string, error) { return GenerateCompletion() },
	"20-history":    func(ctx context.Context) (string, error) { return GenerateHistory() },
}

// Generate executes all generators in parallel and returns ordered output.
// Requires a taskgroup Session/Group on ctx (CLI root installs one).
func Generate(ctx context.Context) (string, error) {
	profile := os.Getenv("WORKSPACED_PROFILE") == "1"

	keys := make([]string, 0, len(generators))
	for k := range generators {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	type genOut struct {
		output   string
		err      error
		duration time.Duration
	}

	// Soft-collect per-generator errors so we can errors.Join them all
	// (Map itself is first-error-wins if Fn returns err).
	outs, err := taskgroup.Map[string, genOut]{
		Name:     "shellgen",
		Items:    keys,
		PoolKind: taskgroup.Control,
		TaskName: func(_ int, key string) string { return "gen:" + key },
		Fn: func(ctx context.Context, s *taskgroup.Status, key string) (genOut, error) {
			start := time.Now()
			output, err := generators[key](ctx)
			return genOut{output: output, err: err, duration: time.Since(start)}, nil
		},
	}.Run(ctx)
	if err != nil {
		return "", err
	}

	var errs []error
	if profile {
		logger := logging.GetLogger(ctx)
		for i, key := range keys {
			if outs[i].err != nil {
				continue
			}
			logger.Info("shell generator timing", "generator", key, "duration", outs[i].duration)
		}
	}

	var output strings.Builder
	for i, key := range keys {
		o := outs[i]
		if o.err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, o.err))
			continue
		}
		fmt.Fprintf(&output, "# Generated: %s\n", key)
		output.WriteString(o.output)
		if !strings.HasSuffix(o.output, "\n") {
			output.WriteString("\n")
		}
		output.WriteString("\n")
	}

	if len(errs) > 0 {
		return "", errors.Join(errs...)
	}
	return output.String(), nil
}
