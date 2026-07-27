package history

import (
	"encoding/json"
	"github.com/lucasew/workspaced/internal/db"
	"github.com/lucasew/workspaced/internal/types"
	"github.com/lucasew/workspaced/pkg/logging"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		cmd := &cobra.Command{
			Use:   "record",
			Short: "Record a command in history",
			RunE: func(c *cobra.Command, args []string) error {
				var event types.HistoryEvent

				// Try reading from stdin if no command flag is provided
				command, err := c.Flags().GetString("command")
				if err != nil {
					return err
				}
				if command == "" {
					if err := json.NewDecoder(os.Stdin).Decode(&event); err != nil {
						return err
					}
				} else {
					event.Command = command
					var ferr error
					if event.Cwd, ferr = c.Flags().GetString("cwd"); ferr != nil {
						return ferr
					}
					if event.ExitCode, ferr = c.Flags().GetInt("exit-code"); ferr != nil {
						return ferr
					}
					if event.Timestamp, ferr = c.Flags().GetInt64("timestamp"); ferr != nil {
						return ferr
					}
					if event.Duration, ferr = c.Flags().GetInt64("duration"); ferr != nil {
						return ferr
					}
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

				if database, ok := db.FromContext(c.Context()); ok {
					return database.RecordHistory(c.Context(), event)
				}

				if err := sendHistoryEvent(c.Context(), event); err == nil {
					return nil
				}

				// Fallback: write to database directly if daemon is not available
				database, err := db.Open(c.Context())
				if err != nil {
					return err
				}
				defer logging.Close(c.Context(), database)
				return database.RecordHistory(c.Context(), event)
			},
		}
		cmd.Flags().String("command", "", "Command string")
		cmd.Flags().String("cwd", "", "Current working directory")
		cmd.Flags().Int("exit-code", 0, "Exit code")
		cmd.Flags().Int64("timestamp", 0, "Timestamp")
		cmd.Flags().Int64("duration", 0, "Duration in ms")
		c.AddCommand(cmd)

	})
}
