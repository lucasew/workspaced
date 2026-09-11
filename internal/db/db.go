// Package db is the workspaced sqlite store.
//
// Layout matches lewkit x/db:
//
//	sqlite/*.sql              // sqlc queries
//	sqlite/migrations/*.sql   // golang-migrate
package db

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lewtec/lewkit/x/db"
	_ "github.com/lewtec/lewkit/x/db/sqlite"
	"github.com/lucasew/workspaced/internal/db/sqlc"
	"github.com/lucasew/workspaced/internal/types"
	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
)

//go:embed sqlite
var FS embed.FS

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
	conn    *db.Conn[*sqlc.Queries]
	Queries *sqlc.Queries
}

func newQueries(tx db.DBTX) *sqlc.Queries {
	return sqlc.New(tx)
}

func Open(ctx context.Context) (*DB, error) {
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
	var a db.Arg[*sqlc.Queries]
	if err := a.Parse(url); err != nil {
		return nil, err
	}
	if err := a.Open(ctx, FS, newQueries); err != nil {
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
	return d.Queries.RecordHistory(ctx, sqlc.RecordHistoryParams{
		Command:    event.Command,
		Cwd:        event.Cwd,
		Timestamp:  event.Timestamp,
		ExitCode:   int64(event.ExitCode),
		DurationMs: event.Duration,
	})
}

func (d *DB) BatchRecordHistory(ctx context.Context, events []types.HistoryEvent) error {
	return d.conn.Tx(ctx, func(q *sqlc.Queries) error {
		for _, event := range events {
			if err := q.RecordHistory(ctx, sqlc.RecordHistoryParams{
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
	var rows []sqlc.History
	var err error
	limit64 := int64(limit)
	if query == "" {
		rows, err = d.Queries.GetHistory(ctx, limit64)
	} else {
		rows, err = d.Queries.SearchHistory(ctx, sqlc.SearchHistoryParams{
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
