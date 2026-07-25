package sudo

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lucasew/workspaced/internal/types"
	"github.com/lucasew/workspaced/pkg/driver/notification"
	"github.com/lucasew/workspaced/pkg/logging"
)

func getQueueDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	// Owner-only: queue files carry full process env (secrets).
	dir := filepath.Join(home, ".cache/workspaced/sudo_queue")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// queuePath resolves slug to a file under the queue dir. Rejects empty,
// path separators, ".." components, and any cleaned result that would escape.
func queuePath(dir, slug string) (string, error) {
	if slug == "" {
		return "", fmt.Errorf("sudo queue slug is empty")
	}
	if strings.Contains(slug, string(os.PathSeparator)) || strings.Contains(slug, "/") || strings.Contains(slug, "\\") {
		return "", fmt.Errorf("sudo queue slug must be a single path element: %q", slug)
	}
	if slug == "." || slug == ".." || strings.Contains(slug, "..") {
		return "", fmt.Errorf("sudo queue slug must be a single path element: %q", slug)
	}
	// filepath.Base rejects multi-segment after Clean; still join carefully.
	base := filepath.Base(slug)
	if base != slug || base == "." || base == ".." {
		return "", fmt.Errorf("sudo queue slug must be a single path element: %q", slug)
	}
	path := filepath.Join(dir, base+".json")
	// Ensure the result stays under dir even if Base were to misbehave.
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("sudo queue slug escapes queue dir: %q", slug)
	}
	return path, nil
}

func Enqueue(ctx context.Context, cmd *types.SudoCommand) error {
	if cmd.Slug == "" {
		b := make([]byte, 3)
		if _, err := rand.Read(b); err != nil {
			return err
		}
		cmd.Slug = fmt.Sprintf("%x", b)
	}

	if cmd.Cwd == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		cmd.Cwd = cwd
	}

	if len(cmd.Env) == 0 {
		cmd.Env = os.Environ()
	}

	if cmd.Timestamp == 0 {
		cmd.Timestamp = time.Now().Unix()
	}

	dir, err := getQueueDir()
	if err != nil {
		return err
	}

	path, err := queuePath(dir, cmd.Slug)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cmd, "", "  ")
	if err != nil {
		return err
	}

	// Atomic owner-only write: env may include tokens/passwords.
	tmp, err := os.CreateTemp(dir, ".sudo-queue-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	ok = true

	n := &notification.Notification{
		Title:   "Sudo Required",
		Message: fmt.Sprintf("Command '%s' (slug: %s) pending approval.", cmd.Command, cmd.Slug),
		Icon:    "dialog-password",
	}
	logging.ReportError(ctx, notification.Notify(ctx, n))

	return nil
}

func List(ctx context.Context) ([]*types.SudoCommand, error) {
	dir, err := getQueueDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var cmds []*types.SudoCommand
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				logging.ReportError(ctx, err)
				continue
			}
			var cmd types.SudoCommand
			if err := json.Unmarshal(data, &cmd); err != nil {
				logging.ReportError(ctx, err)
				continue
			}
			cmds = append(cmds, &cmd)
		}
	}
	return cmds, nil
}

func Get(slug string) (*types.SudoCommand, error) {
	dir, err := getQueueDir()
	if err != nil {
		return nil, err
	}

	path, err := queuePath(dir, slug)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cmd types.SudoCommand
	if err := json.Unmarshal(data, &cmd); err != nil {
		return nil, err
	}
	return &cmd, nil
}

func Remove(slug string) error {
	dir, err := getQueueDir()
	if err != nil {
		return err
	}
	path, err := queuePath(dir, slug)
	if err != nil {
		return err
	}
	return os.Remove(path)
}
