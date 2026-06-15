package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
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

		s.Update("fetching " + name)

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
			resp.Body = &progressReadCloser{
				ReadCloser:   resp.Body,
				s:            s,
				total:        total,
				name:         name,
				completionCh: bodyComplete,
			}
		} else {
			// Unknown size (no Content-Length). Keep the task visible with a
			// running byte counter. The task will complete when body is closed.
			s.Progress(0, 1)
			resp.Body = &progressReadCloser{
				ReadCloser:   resp.Body,
				s:            s,
				total:        0,
				name:         name,
				completionCh: bodyComplete,
			}
		}

		// Hand the response back to the original caller of Do()/RoundTrip.
		// From this point the caller owns the (possibly wrapped) Body.
		resCh <- result{resp: resp, err: nil}

		// Park here until the *caller* has fully consumed or closed the body.
		// This keeps the Internet task "in flight" for the duration of the
		// transfer, which is what we want for the UI.
		<-bodyComplete

		if total > 0 {
			s.Progress(total, total)
		} else {
			s.Progress(1, 1)
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
			// Throttle the message updates.
			if (p.written%(64*1024) == 0) || (p.total > 0 && p.written >= p.total) {
				pct := int(100 * p.written / p.total)
				p.s.Update(fmt.Sprintf("fetching %s (%d%%)", p.name, pct))
			}
		} else {
			// Unknown total: at least show increasing bytes so the task stays alive.
			p.s.Progress(p.written, 0)
			if p.written%(128*1024) == 0 {
				p.s.Update(fmt.Sprintf("fetching %s (%d bytes)", p.name, p.written))
			}
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

// taskName chooses a friendly name for the internet task.
// It prefers "fetch:<basename>" for typical archive/asset downloads so the UI
// shows something like "fetch:flutter_linux_3.44.2-stable.tar.xz".
func taskName(req *http.Request) string {
	if req == nil || req.URL == nil {
		return "http:request"
	}
	if base := filepath.Base(req.URL.Path); base != "" && base != "." && base != "/" {
		return "fetch:" + base
	}
	return "http:" + req.URL.Host
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
