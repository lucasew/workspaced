package history

import (
	"encoding/json"
	"fmt"
	"time"
	"workspaced/pkg/db"
	"workspaced/pkg/types"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		cmd := &cobra.Command{
			Use:   "list",
			Short: "List history entries (internal use)",
			RunE: func(c *cobra.Command, args []string) error {
				limit, _ := c.Flags().GetInt32("limit")
				asJSON, _ := c.Flags().GetBool("json")

				database, ok := c.Context().Value(types.DBKey).(*db.DB)
				if !ok {
					var err error
					database, err = db.Open()
					if err != nil {
						return err
					}
					defer database.Close()
				}

				events, err := database.SearchHistory(c.Context(), "", int(limit))

				if err != nil {
					return err
				}

				if asJSON {
					return json.NewEncoder(c.OutOrStdout()).Encode(events)
				}

				for _, e := range events {
					t := time.Unix(e.Timestamp, 0).Format("2006-01-02 15:04:05")
					fmt.Fprintf(c.OutOrStdout(), "%s\t%s\n", t, e.Command)
				}

				return nil
			},
		}
		cmd.Flags().Int32("limit", 5000, "Limit number of entries")
		cmd.Flags().Bool("json", false, "Output as JSON")
		c.AddCommand(cmd)
	})
}
