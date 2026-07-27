package github

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func TestAPIErrorFromResponse(t *testing.T) {
	t.Parallel()

	const requestURL = "https://api.github.com/repos/o/r/releases"

	t.Run("rate limit when body readable", func(t *testing.T) {
		t.Parallel()
		resp := &http.Response{
			Status:     "403 Forbidden",
			StatusCode: http.StatusForbidden,
			Body:       io.NopCloser(strings.NewReader(`{"message":"API rate limit exceeded"}`)),
		}
		err := apiErrorFromResponse(requestURL, resp)
		if !errors.Is(err, ErrAPIError) {
			t.Fatalf("expected ErrAPIError, got %v", err)
		}
		if !errors.Is(err, ErrAPIRateLimit) {
			t.Fatalf("expected ErrAPIRateLimit, got %v", err)
		}
	})

	t.Run("includes body message", func(t *testing.T) {
		t.Parallel()
		resp := &http.Response{
			Status:     "404 Not Found",
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader("Not Found")),
		}
		err := apiErrorFromResponse(requestURL, resp)
		if !errors.Is(err, ErrAPIError) {
			t.Fatalf("expected ErrAPIError, got %v", err)
		}
		if errors.Is(err, ErrAPIRateLimit) {
			t.Fatalf("unexpected rate limit on 404: %v", err)
		}
	})

	t.Run("status only when body empty", func(t *testing.T) {
		t.Parallel()
		resp := &http.Response{
			Status:     "500 Internal Server Error",
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("")),
		}
		err := apiErrorFromResponse(requestURL, resp)
		if !errors.Is(err, ErrAPIError) {
			t.Fatalf("expected ErrAPIError, got %v", err)
		}
		if errors.Is(err, ErrAPIRateLimit) {
			t.Fatalf("unexpected rate limit on empty body: %v", err)
		}
	})

	t.Run("read failure falls back to status and reports read error", func(t *testing.T) {
		t.Parallel()
		readErr := errors.New("boom")
		resp := &http.Response{
			Status:     "502 Bad Gateway",
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(errReader{err: readErr}),
		}
		err := apiErrorFromResponse(requestURL, resp)
		if !errors.Is(err, ErrAPIError) {
			t.Fatalf("expected ErrAPIError, got %v", err)
		}
		if !errors.Is(err, readErr) {
			t.Fatalf("expected wrapped read error, got %v", err)
		}
		// Must not claim rate limit when body was unreadable.
		if errors.Is(err, ErrAPIRateLimit) {
			t.Fatalf("rate-limit path must not trigger without body: %v", err)
		}
	})
}
