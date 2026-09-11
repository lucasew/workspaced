package history

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/db"
	"github.com/lucasew/workspaced/internal/types"
	"github.com/lucasew/workspaced/pkg/logging"
)

type Record struct {
	Command   cmd.StringArg     `long:"command" help:"Command string"`
	Cwd       cmd.StringArg     `long:"cwd" help:"Current working directory"`
	ExitCode  cmd.IntArg[int]   `long:"exit-code" help:"Exit code"`
	Timestamp cmd.IntArg[int64] `long:"timestamp" help:"Timestamp"`
	Duration  cmd.IntArg[int64] `long:"duration" help:"Duration in ms"`
}

func (Record) Description() string { return "Record a command in history" }

func (r *Record) Run(ctx context.Context) error {
	var event types.HistoryEvent

	command := r.Command.Value()
	if command == "" {
		if err := json.NewDecoder(os.Stdin).Decode(&event); err != nil {
			return err
		}
	} else {
		event.Command = command
		event.Cwd = r.Cwd.Value()
		event.ExitCode = r.ExitCode.Value()
		event.Timestamp = r.Timestamp.Value()
		event.Duration = r.Duration.Value()
	}

	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}
	if event.Cwd == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		event.Cwd = cwd
	}

	if database, ok := db.FromContext(ctx); ok {
		return database.RecordHistory(ctx, event)
	}

	if err := sendHistoryEvent(ctx, event); err == nil {
		return nil
	}

	database, err := db.Open(ctx)
	if err != nil {
		return err
	}
	defer logging.Close(ctx, database)
	return database.RecordHistory(ctx, event)
}
