package history

import (
	"errors"
	"fmt"
	"log/slog"
	"workspaced/pkg/db"
	"workspaced/pkg/types"

	"github.com/spf13/cobra"
)

var ErrUnknownSource = errors.New("unknown source")

func init() {
	Registry.Register(func(c *cobra.Command) {
		c.AddCommand(&cobra.Command{
			Use:   "ingest [source]",
			Short: "Ingest history from other sources (bash, atuin)",
			Args:  cobra.ExactArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				source := args[0]
				database, ok := c.Context().Value(types.DBKey).(*db.DB)
				if !ok {
					var err error
					database, err = db.Open()
					if err != nil {
						return err
					}
					defer database.Close()
				}

				var events []types.HistoryEvent
				var err error

				switch source {
				case "bash":
					events, err = ingestBash()
				case "atuin":
					events, err = ingestAtuin()
				default:
					return fmt.Errorf("%w: %s", ErrUnknownSource, source)
				}

				if err != nil {
					return err
				}

				if len(events) == 0 {
					slog.Info("No events to ingest")
					return nil
				}

				slog.Info("Ingesting events...", "amount", len(events))
				return database.BatchRecordHistory(c.Context(), events)
			},
		})

	})
}
