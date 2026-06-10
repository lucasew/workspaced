package fetchurl

import (
	"context"
	"fmt"
	"path/filepath"

	"workspaced/pkg/driver"
	fetchurldriver "workspaced/pkg/driver/fetchurl"
	"workspaced/pkg/driver/httpclient"
	"workspaced/pkg/taskgroup"

	"github.com/lucasew/fetchurl"
)

func init() {
	driver.Register[fetchurldriver.Driver](&Provider{})
}

type Provider struct{}

func (p *Provider) ID() string   { return "fetchurl" }
func (p *Provider) Name() string { return "fetchurl" }

func (p *Provider) CheckCompatibility(ctx context.Context) error {
	// fetchurl is a pure Go library, always compatible
	return nil
}

func (p *Provider) New(ctx context.Context) (fetchurldriver.Driver, error) {
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
		return fmt.Errorf("no URLs provided")
	}
	if opts.Out == nil {
		return fmt.Errorf("no output writer provided")
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
	g.Go(name, taskgroup.Internet, func(cctx context.Context, s *taskgroup.Status) error {
		s.Update("fetching " + name)
		s.Progress(0, -1) // the fetch task itself provides the progress item

		fetchOpts := fetchurl.FetchOptions{
			URLs: opts.URLs,
			Algo: opts.Algo,
			Hash: opts.Hash,
			Out:  opts.Out,
		}
		err := d.fetcher.Fetch(cctx, fetchOpts)
		s.Progress(1, 1)
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
