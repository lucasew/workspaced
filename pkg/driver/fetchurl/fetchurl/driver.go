package fetchurl

import (
	"context"
	"fmt"

	"workspaced/pkg/driver"
	fetchurldriver "workspaced/pkg/driver/fetchurl"
	"workspaced/pkg/driver/httpclient"

	"github.com/lucasew/fetchurl"
)

func init() {
	driver.Register[fetchurldriver.Driver](&Provider{})
}

type Provider struct{}

func (p *Provider) ID() string         { return "fetchurl" }
func (p *Provider) Name() string       { return "fetchurl" }
func (p *Provider) DefaultWeight() int { return driver.DefaultWeight }

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

	fetchOpts := fetchurl.FetchOptions{
		URLs: opts.URLs,
		Algo: opts.Algo,
		Hash: opts.Hash,
		Out:  opts.Out,
	}

	return d.fetcher.Fetch(ctx, fetchOpts)
}
