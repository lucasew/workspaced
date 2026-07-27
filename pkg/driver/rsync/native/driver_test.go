package native

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/lucasew/workspaced/pkg/logging"
)

func TestExecRsyncBinaryNotAvailable(t *testing.T) {
	t.Parallel()
	// No exec driver on ctx → IsBinaryAvailable is false.
	ctx := logging.NewWriterContext(t.Output())
	err := (&Driver{}).execRsync(ctx, []string{"-av", "a/", "b/"}, nil, nil, slog.Default())
	if !errors.Is(err, ErrBinaryNotAvailable) {
		t.Fatalf("err=%v want errors.Is(..., ErrBinaryNotAvailable)", err)
	}
}
