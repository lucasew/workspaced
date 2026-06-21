package backup_test

import (
	"errors"
	"log/slog"
	"testing"

	"workspaced/pkg/backup"
	"workspaced/pkg/logging"
)

func TestArchiveAction_RunValidation(t *testing.T) {
	t.Parallel()

	ctx := logging.ContextWithLogger(t.Context(), slog.Default())

	tests := []struct {
		name    string
		action  backup.ArchiveAction
		wantErr error
	}{
		{
			name:    "missing input and output",
			action:  backup.ArchiveAction{},
			wantErr: backup.ErrArchiveNeedsInputAndOutput,
		},
		{
			name: "unsupported format",
			action: backup.ArchiveAction{
				InputDir: "/tmp/in",
				Output:   "/tmp/out.tar",
				Format:   "zip",
			},
			wantErr: backup.ErrUnsupportedArchiveFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.action.Run(ctx, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v, want %v", err, tt.wantErr)
			}
		})
	}
}
