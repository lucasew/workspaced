package wallpaper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"workspaced/internal/configcue"
	"workspaced/pkg/api"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/httpclient"
	"workspaced/pkg/logging"
)

func SetStatic(ctx context.Context, path string) error {
	logger := logging.GetLogger(ctx)
	if path == "" {
		cfg, err := configcue.LoadForWorkspace(ctx, "")
		if err != nil {
			return err
		}
		var wallpaper struct {
			Dir string `json:"dir"`
		}
		if err := cfg.Decode("desktop.wallpaper", &wallpaper); err != nil {
			return err
		}
		wallpaperDir := wallpaper.Dir
		files, err := filepath.Glob(filepath.Join(wallpaperDir, "*"))
		if err != nil {
			return fmt.Errorf("error when listing wallpaper candidates: %w", err)
		}
		if len(files) == 0 {
			return fmt.Errorf("%w: wallpapers in %s", api.ErrNoTargetFound, wallpaperDir)
		}
		path = files[rand.Intn(len(files))]
	}

	logger.Info("setting wallpaper", "path", path)

	// Stop existing wallpaper-change service if it exists (best-effort).
	// Missing/not-loaded unit is expected; unexpected stop failures are reported.
	if err := execdriver.MustRun(ctx, "systemctl", "--user", "stop", "wallpaper-change.service").Run(); err != nil {
		msg := strings.ToLower(err.Error())
		// systemctl exit 5 = unit not installed/loaded; stderr often says "not found" / "not loaded"
		if !strings.Contains(msg, "not found") &&
			!strings.Contains(msg, "could not be found") &&
			!strings.Contains(msg, "not loaded") &&
			!strings.Contains(msg, "exit status 5") {
			logging.ReportError(ctx, err, "op", "systemctl --user stop wallpaper-change.service")
		}
	}

	d, err := driver.Get[Driver](ctx)
	if err != nil {
		return err
	}
	return d.SetStatic(ctx, path)
}

func SetAnimated(ctx context.Context, path string) error {
	// Simple wrapper for the video logic
	// Since it uses xrandr and xwinwrap, it's mostly X11
	// We'll just run it via execdriver.MustRun
	return execdriver.MustRun(ctx, "sd", "wall", "video", path).Run()
}

type APODResponse struct {
	HDURL string `json:"hdurl"`
	URL   string `json:"url"`
}

func SetAPOD(ctx context.Context) error {
	logger := logging.GetLogger(ctx)
	apiKey := os.Getenv("NASA_API_KEY")
	if apiKey == "" {
		apiKey = "DEMO_KEY"
	}

	logger.Info("fetching NASA Astronomy Picture of the Day")

	httpDriver, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return err
	}

	apiURL := fmt.Sprintf("https://api.nasa.gov/planetary/apod?api_key=%s", apiKey)
	resp, err := httpDriver.Client().Get(apiURL)
	if err != nil {
		return err
	}
	defer logging.Close(ctx, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", apiURL, resp.Status)
	}

	var apod APODResponse
	if err := json.NewDecoder(resp.Body).Decode(&apod); err != nil {
		return err
	}

	url := apod.HDURL
	if url == "" {
		url = apod.URL
	}
	if url == "" {
		return fmt.Errorf("APOD response missing image URL")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cacheDir := filepath.Join(home, ".cache/workspaced")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	outPath := filepath.Join(cacheDir, "apod.jpg")

	imgResp, err := httpDriver.Client().Get(url)
	if err != nil {
		return err
	}
	defer logging.Close(ctx, imgResp.Body)
	if imgResp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, imgResp.Status)
	}

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, imgResp.Body); err != nil {
		logging.Close(ctx, out)
		_ = os.Remove(outPath)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(outPath)
		return err
	}

	return SetStatic(ctx, outPath)
}
