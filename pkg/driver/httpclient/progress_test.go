package httpclient

import (
	"net/http"
	"net/url"
	"testing"
)

func TestByteProgress(t *testing.T) {
	got := byteProgress(512*1024, 2*1024*1024)
	want := "512.0 KiB / 2.0 MiB"
	if got != want {
		t.Fatalf("byteProgress = %q, want %q", got, want)
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{-1, "0 B"},
		{500, "500 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
	}
	for _, tt := range tests {
		if got := humanBytes(tt.in); got != tt.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTaskName(t *testing.T) {
	req := func(raw string) *http.Request {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		return &http.Request{URL: u}
	}

	tests := []struct {
		raw  string
		want string
	}{
		{"https://cdn.example.com/path/bundle.tar.gz", "bundle.tar.gz"},
		{"https://api.github.com/repos/o/r/releases/assets/12345", "github release"},
		{"https://objects.githubusercontent.com/github-production-release-asset-2e65be/abc", "github"},
		{"https://example.com/api/v1/items/99", "example.com"},
		{"https://example.com/", "example.com"},
	}
	for _, tt := range tests {
		got := taskName(req(tt.raw))
		if got != tt.want {
			t.Errorf("taskName(%s) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestTaskNameWithLabel(t *testing.T) {
	u, _ := url.Parse("https://api.github.com/repos/o/r/releases/assets/1")
	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	req = req.WithContext(WithTaskLabel(req.Context(), "tool@1.2.3"))
	if got := taskName(req); got != "tool@1.2.3" {
		t.Fatalf("taskName with label = %q, want tool@1.2.3", got)
	}
}
