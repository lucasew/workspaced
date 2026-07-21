package taskgroup

import (
	"context"
	"log/slog"
	"testing"

	"github.com/lucasew/workspaced/internal/cmdctx"
	"github.com/lucasew/workspaced/pkg/logging"
)

func TestSessionOverlayVisibleInTasks(t *testing.T) {
	base := logging.ContextWithLogger(t.Context(), slog.Default())
	sess, ctx := Enter(base, DefaultLimits())
	defer sess.Close()

	// After Enter, set dry-run like home plan does.
	ctx = cmdctx.WithDryRun(ctx, true)
	sess.Overlay(ctx)

	var sawDryRun bool
	g := MustFromContext(ctx)
	g.Go("check", Control, func(ctx context.Context, s *Status) error {
		sawDryRun = cmdctx.IsDryRun(ctx)
		return nil
	})
	if err := g.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if !sawDryRun {
		t.Fatal("task ctx missing dry-run from Session.Overlay")
	}
}
