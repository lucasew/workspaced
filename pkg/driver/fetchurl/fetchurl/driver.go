package fetchurl

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/lucasew/fetchurl"
	"workspaced/pkg/driver"
	fetchurldriver "workspaced/pkg/driver/fetchurl"
	"workspaced/pkg/driver/httpclient"
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
	// Get HTTP client from httpclient driver (with proper DNS/certs for Termux)
	var client *http.Client
	if httpDriver, err := driver.Get[httpclient.Driver](ctx); err == nil {
		client = httpDriver.Client()
	}
	// If httpclient driver not available, fetchurl will create default client

	// Get mirror servers from environment or use defaults
	servers := getServersFromEnv()
	return &Driver{
		fetcher: fetchurl.NewFetcher(client, servers),
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

// getServersFromEnv reads FETCHURL_SERVERS environment variable
// Expected format: comma-separated URLs
// Example: FETCHURL_SERVERS=https://mirror1.example.com,https://mirror2.example.com
func getServersFromEnv() []string {
	env := os.Getenv("FETCHURL_SERVERS")
	if env == "" {
		return nil
	}

	servers := strings.Split(env, ",")
	// Trim whitespace from each server URL
	for i := range servers {
		servers[i] = strings.TrimSpace(servers[i])
	}
	return servers
}
