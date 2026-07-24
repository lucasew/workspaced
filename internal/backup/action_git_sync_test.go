package backup

import (
	"testing"

	"github.com/lucasew/workspaced/pkg/logging"
)

func TestGitRepoSyncActionHasHEAD(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	action := GitRepoSyncAction{Src: t.TempDir()}

	if err := action.run(ctx, "init", "--quiet"); err != nil {
		t.Fatalf("git init: %v", err)
	}

	hasHEAD, err := action.hasHEAD(ctx)
	if err != nil {
		t.Fatalf("check unborn HEAD: %v", err)
	}
	if hasHEAD {
		t.Fatal("unborn repository unexpectedly has HEAD")
	}

	if err := action.run(ctx,
		"-c", "user.email=test@example.com",
		"-c", "user.name=Test User",
		"commit", "--quiet", "--allow-empty", "-m", "initial"); err != nil {
		t.Fatalf("create initial commit: %v", err)
	}

	hasHEAD, err = action.hasHEAD(ctx)
	if err != nil {
		t.Fatalf("check committed HEAD: %v", err)
	}
	if !hasHEAD {
		t.Fatal("committed repository has no HEAD")
	}
}
