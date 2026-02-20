package history

import (
	"encoding/json"
	"os"
	"time"
	"workspaced/pkg/db"
	"workspaced/pkg/types"

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
				command, _ := c.Flags().GetString("command")
				if command == "" {
					if err := json.NewDecoder(os.Stdin).Decode(&event); err != nil {
						return err
					}
				} else {
					event.Command = command
					event.Cwd, _ = c.Flags().GetString("cwd")
					event.ExitCode, _ = c.Flags().GetInt("exit-code")
					event.Timestamp, _ = c.Flags().GetInt64("timestamp")
					event.Duration, _ = c.Flags().GetInt64("duration")
				}

				if event.Timestamp == 0 {
					event.Timestamp = time.Now().Unix()
				}
				if event.Cwd == "" {
					event.Cwd, _ = os.Getwd()
				}

				if database, ok := c.Context().Value(types.DBKey).(*db.DB); ok {
					return database.RecordHistory(c.Context(), event)
				}

				if err := sendHistoryEvent(event); err == nil {
					return nil
				}

				// Fallback: write to database directly if daemon is not available
				database, err := db.Open()
				if err != nil {
					return nil // Give up silently to avoid hanging shell
				}
				defer database.Close()
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
