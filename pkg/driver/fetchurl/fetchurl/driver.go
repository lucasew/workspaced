package fetchurl

import (
	"context"
	"fmt"

	"workspaced/pkg/driver"
	fetchurldriver "workspaced/pkg/driver/fetchurl"
	"workspaced/pkg/driver/httpclient"
	"workspaced/pkg/logging"

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
	c := httpclient.WithLogging(httpDriver).Client()
	return &Driver{
		fetcher: fetchurl.NewFetcher(c),
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

	// All HTTP work performed by the underlying library goes through the
	// *http.Client we built in New(). That client has its Transport wrapped
	// (via httpclient.WithProgress in the platform providers) so that every
	// request is automatically treated as an Internet task with progress
	// derived from the real response headers (ContentLength) + bytes read
	// from the body.
	//
	// This is the central interception point the user asked for: download
	// progress + task creation logic lives in the httpclient wrapper instead
	// of being duplicated in every downloader.
	//
	// We no longer need to manually spawn tasks here or maintain a separate
	// progressWriter / probeSize for the write side. The transport does the
	// equivalent (and better — it sees the authoritative headers from the
	// actual GET, not a separate HEAD).
	fetchOpts := fetchurl.FetchOptions{
		URLs: opts.URLs,
		Algo: opts.Algo,
		Hash: opts.Hash,
		Out:  opts.Out,
	}

	l := logging.GetLogger(ctx)
	l.Debug("delegating fetch to library (http requests will be promoted to internet tasks by the httpclient progress wrapper when a taskgroup is present in ctx)")

	if err := d.fetcher.Fetch(ctx, fetchOpts); err != nil {
		l.Error("fetch library call failed", "error", err)
		return err
	}

	l.Debug("fetch library call completed successfully")
	return nil
}
