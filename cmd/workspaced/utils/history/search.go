package history

import (
	"fmt"
	"strings"
	"time"
	"workspaced/pkg/db"
	"workspaced/pkg/registry"
	"workspaced/pkg/types"

	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/spf13/cobra"
)

var Registry registry.CommandRegistry

func init() {
	Registry.Register(func(c *cobra.Command) {
		c.AddCommand(&cobra.Command{
			Use:   "search [query]",
			Short: "Search history using fuzzy finder",
			RunE: func(c *cobra.Command, args []string) error {
				database, ok := c.Context().Value(types.DBKey).(*db.DB)
				if !ok {
					var err error
					database, err = db.Open()
					if err != nil {
						return err
					}
					defer database.Close()
				}

				events, err := database.SearchHistory(c.Context(), "", 5000)
				if err != nil {
					return fmt.Errorf("failed to fetch history: %w", err)
				}

				if len(events) == 0 {
					return fmt.Errorf("no history found")
				}

				// 2. Run fuzzy finder
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

				if len(args) > 0 {
					query := strings.Join(args, " ")
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
					if err == fuzzyfinder.ErrAbort {
						return nil
					}
					return fmt.Errorf("fuzzy finder failed: %w", err)
				}

				fmt.Print(strings.TrimSpace(events[idx].Command))
				return nil
			},
		})

	})
}
