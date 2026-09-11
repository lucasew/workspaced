package history

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lucasew/workspaced/internal/db"
	"github.com/lucasew/workspaced/internal/types"
	"github.com/lucasew/workspaced/pkg/logging"
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
				database, ok := db.FromContext(c.Context())
				if !ok {
					var err error
					database, err = db.Open(c.Context())
					if err != nil {
						return err
					}
					defer logging.Close(c.Context(), database)
				}

				var events []types.HistoryEvent
				var err error

				switch source {
				case "bash":
					events, err = ingestBash(c.Context())
				case "atuin":
					events, err = ingestAtuin(c.Context())
				default:
					return fmt.Errorf("%w: %s", ErrUnknownSource, source)
				}

				if err != nil {
					return err
				}

				if len(events) == 0 {
					logger := logging.GetLogger(c.Context())
					logger.Info("No events to ingest")
					return nil
				}

				logger := logging.GetLogger(c.Context())
				logger.Info("Ingesting events...", "amount", len(events))
				return database.BatchRecordHistory(c.Context(), events)
			},
		})

	})
}
