package media

import (
	"context"
	"crypto/md5"
	"fmt"
	"github.com/lucasew/workspaced/pkg/driver"
	"github.com/lucasew/workspaced/pkg/driver/httpclient"
	"github.com/lucasew/workspaced/pkg/logging"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type PlaybackStatus string

const (
	StatusPlaying PlaybackStatus = "Playing"
	StatusPaused  PlaybackStatus = "Paused"
	StatusStopped PlaybackStatus = "Stopped"
)

type Metadata struct {
	Title    string
	Artist   string
	ArtUrl   string
	Length   int64 // in microseconds
	Position int64 // in microseconds
	Status   PlaybackStatus
	Player   string // player name/bus name
}

type Driver interface {
	Next(ctx context.Context) error
	Previous(ctx context.Context) error
	PlayPause(ctx context.Context) error
	Stop(ctx context.Context) error
	GetMetadata(ctx context.Context) (*Metadata, error)
	// Watch blocks and calls callback when metadata changes
	Watch(ctx context.Context, callback func(*Metadata)) error
}

func GetArtCachePath(ctx context.Context, url string) (string, error) {
	if after, ok := strings.CutPrefix(url, "file://"); ok {
		return after, nil
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return url, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(home, ".cache/workspaced/media_art")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}

	hash := fmt.Sprintf("%x", md5.Sum([]byte(url)))
	path := filepath.Join(cacheDir, hash)

	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	// Use httpclient driver for proper DNS/certs handling
	httpDriver, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return "", err
	}

	resp, err := httpDriver.Client().Get(url)
	if err != nil {
		return "", err
	}
	defer logging.Close(ctx, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		logging.Close(ctx, out)
		_ = os.Remove(path)
		return "", err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}

	return path, nil
}
