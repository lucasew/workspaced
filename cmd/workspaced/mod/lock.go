package mod

import (
	"context"

	"workspaced/internal/modfile"
	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		c.AddCommand(&cobra.Command{
			Use:   "lock",
			Short: "Refresh workspaced.lock.json for enabled modules",
			RunE:  runModLock,
		})
	})
}

func runModLock(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	g := taskgroup.MustFromContext(ctx)
	g.Go("mod:lock", taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update("refreshing lockfile")
		// No Unit(): PopulateSourceLockHashes owns the source-locks aggregate bar
		// when sources need resolution; otherwise this finishes immediately.
		logger := logging.GetLogger(ctx)
		ws, err := modfile.DetectWorkspace(ctx, "")
		if err != nil {
			return err
		}
		result, err := modfile.GenerateLock(ctx, ws)
		if err != nil {
			return err
		}
		if result.Changed {
			logger.Info("wrote lockfile", "path", ws.SumPath(), "sources", result.Sources)
		} else {
			logger.Info("lockfile up to date", "path", ws.SumPath(), "sources", result.Sources)
		}
		return nil
	})
	return nil
}
