package backup

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"workspaced/pkg/driver/notification"
	"workspaced/pkg/driver/rsync"
	"workspaced/pkg/logging"
)

func init() {
	registerAction[RsyncAction]("rsync")
}

type RsyncAction struct {
	backupActionBase
	Src             string   `json:"src"`
	Dst             string   `json:"dst"`
	Excludes        []string `json:"excludes"`
	SkipPermissions bool     `json:"skip_permissions"`
}

var (
	ErrRsyncNeedsSrcAndDst = errors.New("rsync requires src and dst")
)

func (a RsyncAction) Run(ctx context.Context, n *notification.Notification) error {
	logger := logging.GetLogger(ctx)

	if strings.TrimSpace(a.Src) == "" || strings.TrimSpace(a.Dst) == "" {
		return ErrRsyncNeedsSrcAndDst
	}
	logger.Info("rsync sync", "from", a.Src, "to", a.Dst)

	// Use a pipe so we can forward rsync output lines to the desktop notification
	// (preserving the previous live-update behavior) while the driver handles
	// taskgroup progress.
	pr, pw := io.Pipe()

	opts := rsync.Options{
		Excludes:        a.Excludes,
		SkipPermissions: a.SkipPermissions,
		Output:          io.MultiWriter(pw, os.Stderr),
	}

	// Scanner goroutine feeds the notification with live rsync status lines.
	scanDone := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(pr)
		lastUpdate := time.Now()
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				n.Message = line
				logger.Debug("rsync", "line", line)
			}
			if time.Since(lastUpdate) > time.Second {
				if (*notification.Notification)(n) != nil {
					logging.ReportError(ctx, notification.Notify(ctx, n))
				}
				lastUpdate = time.Now()
			}
		}
		scanDone <- scanner.Err()
	}()

	err := rsync.Sync(ctx, a.Src, a.Dst, opts)

	// Close write side so the scanner goroutine drains and exits.
	logging.Close(ctx, pw)
	scanErr := <-scanDone

	return errors.Join(err, scanErr)
}
