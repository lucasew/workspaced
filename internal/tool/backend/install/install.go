package install

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"workspaced/internal/constants"
	"workspaced/internal/tool/backend"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/fetchurl"
	"workspaced/pkg/driver/httpclient"
	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"
)

var (
	ErrEmptyDownloadURL = errors.New("download URL cannot be empty")
	ErrNoDownloadURLs   = errors.New("no download URLs provided")
)

type DownloadOptions struct {
	Hash             string
	Size             int64
	Mode             os.FileMode
	ConfigureRequest func(*http.Request)
}

func InstallArtifact(ctx context.Context, artifact backend.Artifact, destDir string, opts DownloadOptions) error {
	if opts.Hash == "" {
		opts.Hash = artifact.Hash
	}
	if opts.Size <= 0 {
		opts.Size = artifact.Size
	}

	tmpDir := destDir + ".tmp"
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return err
	}
	defer logging.RunCleanup(ctx, "remove_all", func() error { return os.RemoveAll(tmpDir) })

	downloadPath := filepath.Join(tmpDir, filepath.Base(artifact.URL))
	if err := DownloadFile(ctx, artifact.URL, downloadPath, opts); err != nil {
		return err
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return err
	}
	if err := Extract(ctx, downloadPath, extractDir); err != nil {
		return fmt.Errorf("extract %s: %w", filepath.Base(artifact.URL), err)
	}
	if err := StripTopLevelDir(extractDir); err != nil {
		return err
	}
	if err := MoveContents(extractDir, destDir); err != nil {
		return err
	}
	return NormalizeInstalledBinaries(destDir)
}

func DownloadFile(ctx context.Context, url, dest string, opts DownloadOptions) error {
	if strings.TrimSpace(url) == "" {
		return ErrEmptyDownloadURL
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	var fetchErr error
	if opts.Hash != "" {
		fetchErr = downloadWithFetchurl(ctx, url, dest, opts)
		if fetchErr == nil {
			return nil
		}
		logger := logging.GetLogger(ctx)
		logger.Warn("fetchurl verified download failed, falling back to direct http download", "url", url, "err", fetchErr)
		_ = os.Remove(dest + ".tmp")
	}

	if err := downloadDirect(ctx, url, dest, opts); err != nil {
		if fetchErr != nil {
			return fmt.Errorf("verified download failed: %w; direct download failed: %w", fetchErr, err)
		}
		return err
	}
	return nil
}

func DownloadFirst(ctx context.Context, urls []string, dest string, opts DownloadOptions) error {
	var errs []string
	for _, url := range urls {
		if strings.TrimSpace(url) == "" {
			continue
		}
		if err := DownloadFile(ctx, url, dest, opts); err == nil {
			return nil
		} else {
			errs = append(errs, fmt.Sprintf("%s: %v", url, err))
		}
	}
	if len(errs) == 0 {
		return ErrNoDownloadURLs
	}
	return fmt.Errorf("all downloads failed: %s", strings.Join(errs, "; "))
}

func Extract(ctx context.Context, src, dest string) error {
	switch {
	case strings.HasSuffix(src, ".zip"):
		return unzip(ctx, src, dest)
	case strings.HasSuffix(src, ".tar.gz"), strings.HasSuffix(src, ".tgz"):
		return untargz(ctx, src, dest)
	case strings.HasSuffix(src, ".tar.xz"), strings.HasSuffix(src, ".txz"):
		return untarxz(ctx, src, dest)
	default:
		return installBinary(ctx, src, dest)
	}
}

func StripTopLevelDir(destPath string) error {
	entries, err := os.ReadDir(destPath)
	if err != nil {
		return err
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return nil
	}

	singleDir := filepath.Join(destPath, entries[0].Name())
	tempDir := destPath + ".strip-tmp"
	if err := os.Rename(singleDir, tempDir); err != nil {
		return err
	}

	tempEntries, err := os.ReadDir(tempDir)
	if err != nil {
		if restoreErr := os.Rename(tempDir, singleDir); restoreErr != nil {
			return fmt.Errorf("%w; restore failed: %w", err, restoreErr)
		}
		return err
	}

	for _, entry := range tempEntries {
		oldPath := filepath.Join(tempDir, entry.Name())
		newPath := filepath.Join(destPath, entry.Name())
		if err := os.Rename(oldPath, newPath); err != nil {
			return err
		}
	}
	return os.Remove(tempDir)
}

func MoveContents(srcDir, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(destDir, entry.Name())
		if err := os.Rename(src, dst); err != nil {
			return err
		}
	}
	return nil
}

func downloadWithFetchurl(ctx context.Context, url, dest string, opts DownloadOptions) error {
	// Note: "name" (basename) used to be for a local progressWriter.
	// The central progressTransport in the httpclient now handles task naming
	// ("fetch:<basename>") and progress automatically.

	fetcher, err := driver.Get[fetchurl.Driver](ctx)
	if err != nil {
		return err
	}

	tmp := dest + ".tmp"
	outFile, err := os.Create(tmp)
	if err != nil {
		return err
	}

	// HTTP progress is owned by httpclient.WithProgress (one fetch bar).
	// Isolate so a failing verified fetchurl attempt does not cancel the parent
	// group before DownloadFile falls back to downloadDirect.
	algo, hash := parseHash(opts.Hash)
	fetchErr := taskgroup.Isolate(ctx, func(ctx context.Context) error {
		return fetcher.Fetch(ctx, fetchurl.FetchOptions{
			URLs: []string{url},
			Algo: algo,
			Hash: hash,
			Out:  outFile,
			Size: opts.Size,
		})
	})

	if closeErr := outFile.Close(); closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if fetchErr != nil {
		_ = os.Remove(tmp)
		return fetchErr
	}
	return finishDownload(tmp, dest, opts.Mode)
}

func downloadDirect(ctx context.Context, url, dest string, opts DownloadOptions) error {
	// Perform the actual GET + write-to-tmp + finishDownload.
	// If the ctx carries a taskgroup (normal case under tool install / apply),
	// the httpclient driver's WithProgress transport will see the request and
	// promote *this* HTTP to exactly one "fetch:<basename>" Internet task,
	// driving real progress via progressReadCloser on body reads.
	// This gives the asset download a single specific task (dispatch point for
	// either the fetchurl path or this direct path) without an extra wrapper
	// layer around it.
	//
	// Isolate around fetchurl attempts (in downloadWithFetchurl) keeps tasks
	// from a failing verified attempt from recording errors on the parent group,
	// so the fallback here can still succeed cleanly.

	tmp := dest + ".tmp"
	outFile, err := os.Create(tmp)
	if err != nil {
		return err
	}

	httpClient, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		logging.Close(ctx, outFile)
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to get http client: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logging.Close(ctx, outFile)
		_ = os.Remove(tmp)
		return err
	}
	if opts.ConfigureRequest != nil {
		opts.ConfigureRequest(req)
	}

	resp, err := httpClient.Client().Do(req)
	if err != nil {
		logging.Close(ctx, outFile)
		_ = os.Remove(tmp)
		return err
	}

	if resp.StatusCode != http.StatusOK {
		logging.Close(ctx, resp.Body)
		logging.Close(ctx, outFile)
		_ = os.Remove(tmp)
		err := fmt.Errorf("GET %s: %s", url, resp.Status)
		if resp.StatusCode == http.StatusForbidden {
			err = fmt.Errorf("%w (if this is a GitHub release asset, set GITHUB_TOKEN or run 'gh auth login' to increase rate limits)", err)
		}
		return err
	}

	// resp.Body is the original (no group) or a progressReadCloser (group present
	// at the time of this Do). io.Copy will drive the single transport-created
	// task's progress when applicable.
	if _, err := io.Copy(outFile, resp.Body); err != nil {
		logging.Close(ctx, resp.Body)
		logging.Close(ctx, outFile)
		_ = os.Remove(tmp)
		return err
	}
	logging.Close(ctx, resp.Body)
	if err := outFile.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return finishDownload(tmp, dest, opts.Mode)
}

func finishDownload(tmp, dest string, mode os.FileMode) error {
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if mode != 0 {
		return os.Chmod(dest, mode)
	}
	return nil
}

func parseHash(raw string) (algo, hash string) {
	algo, hash = "sha256", raw
	if parts := strings.SplitN(raw, ":", 2); len(parts) == 2 {
		algo, hash = parts[0], parts[1]
	}
	return algo, hash
}

func installBinary(ctx context.Context, src, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer logging.Close(ctx, in)

	outPath := filepath.Join(dest, NormalizeBinaryName(filepath.Base(src)))
	out, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer logging.Close(ctx, out)

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return os.Chmod(outPath, 0o755)
}

var versionPattern = regexp.MustCompile(constants.BinaryVersionPattern)

func NormalizeBinaryName(name string) string {
	result := name
	for _, suffix := range constants.BinaryNameSuffixes {
		if before, ok := strings.CutSuffix(result, suffix); ok {
			result = before
			break
		}
	}
	return versionPattern.ReplaceAllString(result, "")
}

// NormalizeInstalledBinaries renames top-level and bin/ executables whose
// basenames still embed platform triples (e.g. codex-x86_64-unknown-linux-musl
// -> codex) using NormalizeBinaryName.
func NormalizeInstalledBinaries(destDir string) error {
	for _, dir := range []string{destDir, filepath.Join(destDir, "bin")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			oldName := entry.Name()
			newName := NormalizeBinaryName(oldName)
			if newName == "" || newName == oldName {
				continue
			}
			oldPath := filepath.Join(dir, oldName)
			info, err := os.Stat(oldPath)
			if err != nil {
				return err
			}
			if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
				continue
			}
			newPath := filepath.Join(dir, newName)
			if _, err := os.Stat(newPath); err == nil {
				continue
			}
			if err := os.Rename(oldPath, newPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func unzip(ctx context.Context, src, dest string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer logging.Close(ctx, reader)

	for _, file := range reader.File {
		target := filepath.Join(dest, file.Name)
		if !isPathWithinDest(dest, target) {
			return fmt.Errorf("illegal file path: %s", target)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, os.ModePerm); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}

		rc, err := file.Open()
		if err != nil {
			logging.Close(ctx, outFile)
			return err
		}

		_, copyErr := io.Copy(outFile, rc)
		closeOutErr := outFile.Close()
		closeRcErr := rc.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeOutErr != nil {
			return closeOutErr
		}
		if closeRcErr != nil {
			return closeRcErr
		}
		if file.Mode()&0o111 != 0 {
			if err := os.Chmod(target, file.Mode()); err != nil {
				return fmt.Errorf("failed to set permissions: %w", err)
			}
		}
	}
	return nil
}

func untargz(ctx context.Context, src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer logging.Close(ctx, file)

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer logging.Close(ctx, gzipReader)

	return untar(ctx, tar.NewReader(gzipReader), dest)
}

func untar(ctx context.Context, reader *tar.Reader, dest string) error {
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)
		if !isPathWithinDest(dest, target) {
			return fmt.Errorf("illegal file path: %s", target)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(outFile, reader)
			closeErr := outFile.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			if header.Mode&0o111 != 0 {
				if err := os.Chmod(target, os.FileMode(header.Mode)); err != nil {
					return fmt.Errorf("failed to set permissions: %w", err)
				}
			}
		}
	}
}

func untarxz(ctx context.Context, src, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}

	cmd := execdriver.MustRun(ctx, "tar", "-xf", src, "-C", dest)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tar xf failed: %w", err)
	}
	return nil
}

func isPathWithinDest(dest, target string) bool {
	cleanDest := filepath.Clean(dest)
	cleanTarget := filepath.Clean(target)

	rel, err := filepath.Rel(cleanDest, cleanTarget)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
