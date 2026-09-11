package history

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/db"
	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

var ErrNoHistory = errors.New("no history found")

type Search struct {
	query []cmd.StringArg
}

func (Search) Description() string { return "Search history using fuzzy finder" }

func (s *Search) Run(ctx context.Context) error {
	database, ok := db.FromContext(ctx)
	if !ok {
		var err error
		database, err = db.Open(ctx)
		if err != nil {
			return err
		}
		defer logging.Close(ctx, database)
	}

	events, err := database.SearchHistory(ctx, "", 5000)
	if err != nil {
		return fmt.Errorf("fetch history: %w", err)
	}

	if len(events) == 0 {
		return ErrNoHistory
	}

	options := []fuzzyfinder.Option{
		fuzzyfinder.WithPreviewWindow(func(i int, width int, height int) string {
			if i == -1 {
				return ""
			}
			e := events[i]
			t := time.Unix(e.Timestamp, 0).Format("2006-01-02 15:04:05")
			return fmt.Sprintf("Time:     %s\nExitCode: %d\nCwd:      %s\nDuration: %dms\n\nCommand:\n%s",
				t, e.ExitCode, e.Cwd, e.Duration, e.Command)
		}),
	}

	if len(s.query) > 0 {
		query := strings.Join(cmd.Values(s.query), " ")
		query = strings.Trim(query, "'\"")
		if query != "" {
			options = append(options, fuzzyfinder.WithQuery(query))
		}
	}

	idx, err := fuzzyfinder.Find(
		events,
		func(i int) string {
			return events[i].Command
		},
		options...,
	)

	if err != nil {
		if errors.Is(err, fuzzyfinder.ErrAbort) {
			return nil
		}
		return fmt.Errorf("fuzzy finder failed: %w", err)
	}

	selected := strings.TrimSpace(events[idx].Command)
	taskgroup.MustSessionFrom(ctx).AfterWait(func() error {
		if selected == "" {
			return nil
		}
		_, err := fmt.Fprint(os.Stdout, selected)
		return err
	})
	return nil
}
