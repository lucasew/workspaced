// Package db is the workspaced sqlite store.
//
// Queries and migrations live under sqlite/. `lewkit generate db internal/db`
// writes sqlc output, FS, Queries, New, and Open.
package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	xdb "github.com/lewtec/lewkit/x/db"
	"github.com/lucasew/workspaced/internal/types"
	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
)

type dbKey struct{}

// WithDB returns a context that carries the given database connection.
func WithDB(ctx context.Context, database *DB) context.Context {
	return context.WithValue(ctx, dbKey{}, database)
}

// FromContext retrieves the database connection from the context.
func FromContext(ctx context.Context) (*DB, bool) {
	database, ok := ctx.Value(dbKey{}).(*DB)
	return database, ok
}

type DB struct {
	conn    *xdb.Conn[Queries]
	Queries Queries
}

// OpenDefault opens the user-data-dir workspaced.db.
func OpenDefault(ctx context.Context) (*DB, error) {
	dataDir, err := envdriver.GetUserDataDir(ctx)
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dataDir, "workspaced.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}
	return OpenURL(ctx, dbPath)
}

// OpenURL opens a sqlite URL (bare path, file:, sqlite:, or :memory:)
// and applies sqlite/migrations.
func OpenURL(ctx context.Context, url string) (*DB, error) {
	var a xdb.Arg[Queries]
	if err := a.Parse(url); err != nil {
		return nil, err
	}
	if err := Open(ctx, &a); err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	conn := a.Value()
	return &DB{conn: conn, Queries: conn.Queries()}, nil
}

func (d *DB) Close() error {
	if d == nil || d.conn == nil {
		return nil
	}
	return d.conn.Close()
}

func (d *DB) RecordHistory(ctx context.Context, event types.HistoryEvent) error {
	return d.Queries.RecordHistory(ctx, RecordHistoryParams{
		Command:    event.Command,
		Cwd:        event.Cwd,
		Timestamp:  event.Timestamp,
		ExitCode:   int64(event.ExitCode),
		DurationMs: event.Duration,
	})
}

func (d *DB) BatchRecordHistory(ctx context.Context, events []types.HistoryEvent) error {
	return d.conn.Tx(ctx, func(q Queries) error {
		for _, event := range events {
			if err := q.RecordHistory(ctx, RecordHistoryParams{
				Command:    event.Command,
				Cwd:        event.Cwd,
				Timestamp:  event.Timestamp,
				ExitCode:   int64(event.ExitCode),
				DurationMs: event.Duration,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *DB) SearchHistory(ctx context.Context, query string, limit int) ([]types.HistoryEvent, error) {
	var rows []History
	var err error
	limit64 := int64(limit)
	if query == "" {
		rows, err = d.Queries.GetHistory(ctx, limit64)
	} else {
		rows, err = d.Queries.SearchHistory(ctx, SearchHistoryParams{
			Command: "%" + query + "%",
			Limit:   limit64,
		})
	}
	if err != nil {
		return nil, err
	}
	events := make([]types.HistoryEvent, len(rows))
	for i, row := range rows {
		events[i] = types.HistoryEvent{
			Command:   row.Command,
			Cwd:       row.Cwd,
			Timestamp: row.Timestamp,
			ExitCode:  int(row.ExitCode),
			Duration:  row.DurationMs,
		}
	}
	return events, nil
}
