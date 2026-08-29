package dotfiles

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/lucasew/workspaced/internal/deployer"
	"github.com/lucasew/workspaced/internal/source"
	"github.com/lucasew/workspaced/pkg/logging"
)

func TestLogApplyResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		result      *ApplyResult
		opts        LogApplyOptions
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:   "nil result is no-op",
			result: nil,
		},
		{
			name:   "home idle apply stays quiet",
			result: &ApplyResult{},
			opts:   LogApplyOptions{},
			wantAbsent: []string{
				"no changes needed",
				"apply summary",
			},
		},
		{
			name:   "codebase idle apply logs target",
			result: &ApplyResult{},
			opts: LogApplyOptions{
				NoChangesTarget: "repo root",
			},
			wantContain: []string{
				"no changes needed",
				`target="repo root"`,
			},
		},
		{
			name:   "codebase idle plan suppresses idle line",
			result: &ApplyResult{},
			opts: LogApplyOptions{
				DryRun:          true,
				NoChangesTarget: "repo root",
			},
			wantAbsent: []string{
				"no changes needed",
			},
		},
		{
			name: "actions and summary hide noop by default",
			result: &ApplyResult{
				FilesCreated: 1,
				FilesUpdated: 1,
				FilesDeleted: 0,
				FilesNoOp:    1,
				Actions: []deployer.Action{
					{
						Type:   deployer.ActionCreate,
						Target: "/tmp/created",
						Desired: deployer.DesiredState{
							File: &source.BufferFile{
								BasicFile: source.BasicFile{Info: "mod:a"},
								Content:   []byte("x"),
							},
						},
					},
					{
						Type:   deployer.ActionUpdate,
						Target: "/tmp/updated",
						Desired: deployer.DesiredState{
							File: &source.BufferFile{
								BasicFile: source.BasicFile{Info: "mod:b"},
								Content:   []byte("y"),
							},
						},
					},
					{
						Type:   deployer.ActionNoop,
						Target: "/tmp/noop",
					},
				},
			},
			opts: LogApplyOptions{},
			wantContain: []string{
				"apply action",
				"type=+",
				"type=*",
				"source=mod:a",
				"source=mod:b",
				"apply summary",
				"created=1",
				"updated=1",
				"deleted=0",
			},
			wantAbsent: []string{
				`type=" "`,
				"noop=",
			},
		},
		{
			name: "show noop includes noop actions and count",
			result: &ApplyResult{
				FilesNoOp: 1,
				Actions: []deployer.Action{
					{Type: deployer.ActionNoop, Target: "/tmp/noop"},
				},
			},
			opts: LogApplyOptions{ShowNoop: true},
			wantContain: []string{
				`type=" "`,
				"apply summary",
				"noop=1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
			ctx := logging.ContextWithLogger(t.Context(), slog.New(h))

			LogApplyResult(ctx, tt.result, tt.opts)

			got := buf.String()
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("log missing %q\ngot:\n%s", want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("log unexpectedly contains %q\ngot:\n%s", absent, got)
				}
			}
		})
	}
}
