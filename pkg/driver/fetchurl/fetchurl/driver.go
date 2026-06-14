package fetchurl

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"workspaced/pkg/driver"
	fetchurldriver "workspaced/pkg/driver/fetchurl"
	"workspaced/pkg/driver/httpclient"
	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"

	"github.com/lucasew/fetchurl"
)

func init() {
	driver.Register[fetchurldriver.Driver](&Factory{})
}

type Factory struct{}

func (p *Factory) ID() string   { return "fetchurl" }
func (p *Factory) Name() string { return "fetchurl" }

func (p *Factory) CheckCompatibility(ctx context.Context) error {
	// fetchurl is a pure Go library, always compatible
	return nil
}

func (p *Factory) New(ctx context.Context) (fetchurldriver.Driver, error) {
	httpDriver, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return nil, fmt.Errorf("httpclient driver required: %w", err)
	}
	return &Driver{
		fetcher: fetchurl.NewFetcher(httpclient.WithLogging(httpDriver).Client()),
	}, nil
}

type Driver struct {
	fetcher *fetchurl.Fetcher
}

func (d *Driver) Fetch(ctx context.Context, opts fetchurldriver.FetchOptions) error {
	if len(opts.URLs) == 0 {
		return fetchurldriver.ErrNoURLs
	}
	if opts.Out == nil {
		return fetchurldriver.ErrNoOutputWriter
	}

	g := taskgroup.FromContext(ctx)
	if g == nil {
		// No group: direct fetch (no task spawned).
		fetchOpts := fetchurl.FetchOptions{
			URLs: opts.URLs,
			Algo: opts.Algo,
			Hash: opts.Hash,
			Out:  opts.Out,
		}
		return d.fetcher.Fetch(ctx, fetchOpts)
	}

	// Spawn the fetch as a proper Internet task so the fetcher has its own
	// progress bar / status in the group system.
	done := make(chan error, 1)
	name := "fetch:" + filepath.Base(opts.URLs[0])
	g.Go(name, taskgroup.Internet, func(ctx context.Context, s *taskgroup.Status) error {
		l := logging.GetLogger(ctx) // always re-fetch inside the inner task block; do not inherit/capture logger var from outer scope
		l.Debug("fetch task starting", "name", name)

		total := opts.Size
		if total <= 0 {
			total = 1 // ensure the task registers with the progress renderer (Total > 0)
		}
		s.Update("fetching " + name)
		s.Progress(0, total)

		// Wrap the output so we can drive incremental progress on the Status
		// owned by this task. The external library only accepts a plain io.Writer
		// and does io.Copy internally; the wrapper observes bytes as they flow.
		fetchOut := opts.Out
		if opts.Out != nil {
			pw := &progressWriter{
				w:     opts.Out,
				s:     s,
				name:  name,
				total: opts.Size,
			}
			fetchOut = pw
		}

		fetchOpts := fetchurl.FetchOptions{
			URLs: opts.URLs,
			Algo: opts.Algo,
			Hash: opts.Hash,
			Out:  fetchOut,
		}
		err := d.fetcher.Fetch(ctx, fetchOpts)
		if err != nil {
			l.Error("fetch task failed", "name", name, "error", err)
		} else {
			l.Debug("fetch task completed", "name", name)
		}
		if opts.Size > 0 {
			s.Progress(opts.Size, opts.Size)
		} else {
			s.Progress(1, 1)
		}
		done <- err
		return err
	})

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// progressWriter wraps an io.Writer and drives taskgroup progress + status
// messages as data is written. It is used inside the spawned fetch task so
// that the "fetch:..." Internet task shows real incremental progress in the
// bubbletea UI (instead of staying indeterminate or invisible).
type progressWriter struct {
	w       io.Writer
	s       *taskgroup.Status
	name    string
	total   int64
	written int64
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p)
	if n > 0 {
		pw.written += int64(n)
		if pw.total > 0 {
			cur := pw.written
			if cur > pw.total {
				cur = pw.total
			}
			pw.s.Progress(cur, pw.total)
			// Update the message periodically so the bar shows "(37%)" style
			// text without spamming on every tiny write. The renderer only
			// re-renders on its own ~100ms tick or explicit refresh anyway.
			if (pw.written%(64*1024) == 0) || (pw.total > 0 && pw.written >= pw.total) {
				pct := int(100 * pw.written / pw.total)
				pw.s.Update(fmt.Sprintf("fetching %s (%d%%)", pw.name, pct))
			}
		} else {
			// Unknown size: at least keep the task "alive" with increasing
			// current so it stays visible as an active Internet task.
			pw.s.Progress(pw.written, 0)
			if pw.written%(128*1024) == 0 {
				pw.s.Update(fmt.Sprintf("fetching %s (%d bytes)", pw.name, pw.written))
			}
		}
	}
	return n, err
}
