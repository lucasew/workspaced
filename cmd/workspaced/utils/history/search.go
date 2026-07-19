package history

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"github.com/lucasew/workspaced/internal/cmdregistry"
	"github.com/lucasew/workspaced/internal/db"
	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/taskgroup"

	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/spf13/cobra"
)

var ErrNoHistory = errors.New("no history found")

var Registry cmdregistry.CommandRegistry

func init() {
	Registry.Register(func(c *cobra.Command) {
		c.AddCommand(&cobra.Command{
			Use:   "search [query]",
			Short: "Search history using fuzzy finder",
			RunE: func(c *cobra.Command, args []string) error {
				// No Group.Go: session UI stays off (lazy). Fuzzyfinder owns the tty.
				// Selection is printed in AfterWait so stdout is clean for Ctrl+R
				// command substitution after session Close restores globals.
				ctx := c.Context()

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
			},
		})

	})
}
