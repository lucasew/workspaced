package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"
)

// progressTransport is an http.RoundTripper that, when a taskgroup.Group is
// present in the request context, automatically promotes the request into a
// first-class Internet task. Byte progress is driven from the response
// ContentLength header + actual body reads.
//
// This centralizes "make network work visible as a task with progress" in the
// HTTP layer instead of duplicating g.Go + progress writers in every downloader
// (fetchurl driver, direct downloads, favicon fetchers, etc.).
type progressTransport struct {
	base http.RoundTripper
}

func (t *progressTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	g := taskgroup.FromContext(req.Context())
	if g == nil {
		return t.base.RoundTrip(req)
	}

	name := taskName(req)

	// Channel used to hand the response (headers) back to the caller of RoundTrip
	// as soon as they are available. The body may still be streaming.
	resCh := make(chan result, 1)

	// Signaled by the progressReadCloser when the caller has finished consuming
	// (or closed) the body. This lets the task handler know when to finalize.
	bodyComplete := make(chan struct{})

	g.Go(name, taskgroup.Internet, func(ctx context.Context, s *taskgroup.Status) error {
		l := logging.GetLogger(ctx)
		l.Debug("http request promoted to internet task", "name", name, "url", req.URL.String())

		s.Update(name)

		// Run the actual request inside the task's context (for cancellation etc.).
		req = req.WithContext(ctx)

		resp, err := t.base.RoundTrip(req)
		if err != nil {
			resCh <- result{resp: nil, err: err}
			return err
		}

		total := resp.ContentLength
		if total > 0 {
			s.Progress(0, total)
			s.Update(fmt.Sprintf("fetching %s (0 / %s)", msgName, humanBytes(total)))
			resp.Body = &progressReadCloser{
				ReadCloser:   resp.Body,
				s:            s,
				total:        total,
				name:         msgName,
				completionCh: bodyComplete,
			}
		} else {
			// Unknown size (no Content-Length). Keep the task visible with a
			// running byte counter. The task will complete when body is closed.
			s.Progress(0, 1)
			s.Update("fetching " + msgName)
			resp.Body = &progressReadCloser{
				ReadCloser:   resp.Body,
				s:            s,
				total:        0,
				name:         msgName,
				completionCh: bodyComplete,
			}
		}

		// Hand the response back to the original caller of Do()/RoundTrip.
		// From this point the caller owns the (possibly wrapped) Body.
		resCh <- result{resp: resp, err: nil}

		// Park here until the *caller* has fully consumed or closed the body,
		// *or* our task ctx is canceled. The latter ensures the promoted task
		// always reaches Done (so bubbletea model can Quit and group wgs can
		// make progress) even if the caller/library never closes the body or
		// in shutdown races. Parent Waits + recordError will cancel the group
		// ctxs, unblocking any stragglers.
		select {
		case <-bodyComplete:
		case <-ctx.Done():
		}

		if total > 0 {
			s.Progress(total, total)
			s.Update(fmt.Sprintf("fetched %s (%s)", msgName, humanBytes(total)))
		} else {
			s.Progress(1, 1)
			s.Update("fetched " + msgName)
		}
		return nil
	})

	// Block until the inner handler has performed the RoundTrip and sent us the
	// response headers (or an error). The body streaming happens after we return.
	select {
	case r := <-resCh:
		return r.resp, r.err
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}
}

type result struct {
	resp *http.Response
	err  error
}

// progressReadCloser wraps the response body and reports progress to the task
// Status as bytes are read. When the body is fully read or Close() is called,
// it signals completion so the owning task handler can finish.
type progressReadCloser struct {
	io.ReadCloser
	s            *taskgroup.Status
	total        int64
	written      int64
	name         string
	completionCh chan struct{}
	once         sync.Once
}

func (p *progressReadCloser) Read(b []byte) (int, error) {
	n, err := p.ReadCloser.Read(b)
	if n > 0 {
		p.written += int64(n)

		cur := p.written
		if p.total > 0 && cur > p.total {
			cur = p.total
		}

		if p.total > 0 {
			p.s.Progress(cur, p.total)
			// Update the description (the text after the bar) on every data read
			// so the x/y MiB (and %) live-updates in the progress bars.
			// The model only snapshots every ~100ms so this is not spammy in UI.
			pct := int(100 * p.written / p.total)
			p.s.Update(fmt.Sprintf("fetching %s (%s / %s, %d%%)", p.name, humanBytes(p.written), humanBytes(p.total), pct))
		} else {
			// Unknown total: at least show increasing bytes so the task stays alive.
			p.s.Progress(p.written, 0)
			p.s.Update(fmt.Sprintf("fetching %s (%s)", p.name, humanBytes(p.written)))
		}
	}

	if err != nil {
		p.signalComplete()
	}
	return n, err
}

func (p *progressReadCloser) Close() error {
	err := p.ReadCloser.Close()
	p.signalComplete()
	return err
}

func (p *progressReadCloser) signalComplete() {
	p.once.Do(func() {
		if p.completionCh != nil {
			close(p.completionCh)
		}
	})
}

// humanBytes returns a human readable size using binary units (MiB etc).
// Used to put x/y size hints into the live task description/message.
func humanBytes(b int64) string {
	if b <= 0 {
		return "0 B"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

type taskLabelKey struct{}

// WithTaskLabel attaches a human-readable progress task name for HTTP requests
// made with this context. When set, it overrides automatic URL-derived labels
// (e.g. opaque GitHub asset IDs).
func WithTaskLabel(ctx context.Context, label string) context.Context {
	if ctx == nil || strings.TrimSpace(label) == "" {
		return ctx
	}
	return context.WithValue(ctx, taskLabelKey{}, strings.TrimSpace(label))
}

// taskName chooses a friendly name for the internet task.
// Prefer WithTaskLabel on the request context. Otherwise:
//   - real filenames (contain ".") → "fetch:name"
//   - GitHub release asset API paths → "github release"
//   - UUIDs / bare IDs → host-based label
func taskName(req *http.Request) string {
	if req != nil {
		if v, ok := req.Context().Value(taskLabelKey{}).(string); ok && v != "" {
			return v
		}
	}
	if req == nil || req.URL == nil {
		return "http request"
	}
	path := req.URL.Path
	host := req.URL.Host
	// GitHub API release assets: /repos/{owner}/{repo}/releases/assets/{id}
	if strings.Contains(host, "github") && strings.Contains(path, "/releases/assets/") {
		return "github release download"
	}
	// objects.githubusercontent.com hashed paths
	if strings.Contains(host, "githubusercontent.com") {
		return "github download"
	}
	if base := filepath.Base(path); base != "" && base != "." && base != "/" {
		if strings.Contains(base, ".") {
			return "download " + base
		}
		if looksLikeUUID(base) || isAllDigits(base) {
			if host != "" {
				return "http " + host
			}
			return "http request"
		}
		return base
	}
	if host != "" {
		return "http " + host
	}
	return "http request"
}

func looksLikeUUID(s string) bool {
	return len(s) == 36 && strings.Count(s, "-") == 4
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// WithProgress wraps the given RoundTripper so that every request made
// through it is automatically promoted to a first-class "Internet" task
// (with determinate progress based on Content-Length + body reads) whenever
// a taskgroup.Group is present in the request's context.
//
// This is the central interception point. All code that goes through the
// official httpclient driver (including the client handed to the external
// fetchurl library, direct downloads, etc.) benefits without having to
// duplicate taskgroup + progress logic everywhere.
func WithProgress(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &progressTransport{base: base}
}
