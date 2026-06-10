package backup

import (
	"context"
	"errors"
	"strings"
	"workspaced/pkg/driver/notification"
	"workspaced/pkg/logging"

	"bufio"
	"time"
	execdriver "workspaced/pkg/driver/exec"
)

func init() {
	registerActionProvider[RsyncAction]("rsync")
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
	cmdCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	extraArgs := make([]string, 0, len(a.Excludes))
	for _, x := range a.Excludes {
		extraArgs = append(extraArgs, "--exclude="+x)
	}
	if a.SkipPermissions {
		extraArgs = append(extraArgs, "--no-perms")
	}
	if strings.TrimSpace(a.Src) == "" || strings.TrimSpace(a.Dst) == "" {
		return ErrRsyncNeedsSrcAndDst
	}
	logger.Info("rsync sync", "from", a.Src, "to", a.Dst)
	args := append(extraArgs, "-avP", a.Src, a.Dst)
	cmd := execdriver.MustRun(cmdCtx, "rsync", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	lastUpdate := time.Now()

	for scanner.Scan() {
		if logging.ReportError(ctx, scanner.Err()) {
			return err
		}
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
	return cmd.Wait()
}
