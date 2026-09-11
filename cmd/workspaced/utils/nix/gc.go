package nix

import (
	"context"

	"github.com/lucasew/workspaced/internal/nix"
)

type GcCleanup struct{}

func (GcCleanup) Description() string { return "Cleanup old Nix profiles by enqueuing rm commands" }

func (*GcCleanup) Run(ctx context.Context) error {
	return nix.CleanupProfiles(ctx)
}
