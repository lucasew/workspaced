package history

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/db"
	"github.com/lucasew/workspaced/pkg/logging"
)

type List struct {
	Limit cmd.IntArg[int32] `long:"limit" help:"Limit number of entries" default:"5000"`
	JSON  cmd.Flag          `long:"json" help:"Output as JSON"`
}

func (List) Description() string { return "List history entries (internal use)" }

func (l *List) Run(ctx context.Context) error {
	database, ok := db.FromContext(ctx)
	if !ok {
		var err error
		database, err = db.Open(ctx)
		if err != nil {
			return err
		}
		defer logging.Close(ctx, database)
	}

	events, err := database.SearchHistory(ctx, "", int(l.Limit.Value()))
	if err != nil {
		return err
	}

	if l.JSON.Value() {
		return json.NewEncoder(os.Stdout).Encode(events)
	}

	for _, e := range events {
		t := time.Unix(e.Timestamp, 0).Format("2006-01-02 15:04:05")
		if _, err := fmt.Fprintf(os.Stdout, "%s\t%s\n", t, e.Command); err != nil {
			return err
		}
	}

	return nil
}
