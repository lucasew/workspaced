package demo

import (
	"context"
	"fmt"
	"time"

	"github.com/lucasew/workspaced/pkg/taskgroup"
)

type Lines struct{}

func (Lines) Description() string {
	return `Three live rows that rewrite in place (CR) until a newline commits them

Each job owns a LineWriter. Carriage returns update that row only;
a newline (or Close) pushes it into the transcript above the bars.

This is the same writer exec.Run attaches to child stderr when a Session is on ctx.`
}

func (*Lines) Run(ctx context.Context) error {
	g := taskgroup.MustFromContext(ctx)
	jobs := []struct {
		name string
		n    int
		tick time.Duration
	}{
		{"alpha", 24, 80 * time.Millisecond},
		{"beta", 16, 120 * time.Millisecond},
		{"gamma", 20, 100 * time.Millisecond},
	}
	for _, job := range jobs {
		g.Go(job.name, taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
			w := taskgroup.LineWriterFrom(ctx)
			defer w.Close()
			for i := 1; i <= job.n; i++ {
				if _, err := fmt.Fprintf(w, "%s %d/%d\r", job.name, i, job.n); err != nil {
					return err
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(job.tick):
				}
			}
			_, err := fmt.Fprintf(w, "%s done\n", job.name)
			return err
		})
	}
	return nil
}
