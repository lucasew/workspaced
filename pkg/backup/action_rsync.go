package backup

import (
	"context"
	"fmt"
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
	Src              string   `json:"src"`
	Dst              string   `json:"dst"`
	Excludes         []string `json:"excludes"`
	SkipPermissions  bool     `json:"skip_permissions"`
}

func (a RsyncAction) Run(ctx context.Context, n *notification.Notification) error {
	extraArgs := make([]string, 0, len(a.Excludes))
	for _, x := range a.Excludes {
		extraArgs = append(extraArgs, "--exclude="+x)
	}
	if a.SkipPermissions {
		extraArgs = append(extraArgs, "--no-perms")
	}
	_, err := func() (string, error) {
		var extraArgs []string = extraArgs
		if strings.TrimSpace(a.Src) == "" || strings.TrimSpace(a.Dst) == "" {
			return "", fmt.Errorf("rsync requires src and dst")
		}
		logging.GetLogger(ctx).Info("rsync sync", "from", a.Src, "to", a.Dst)
		args := append([]string{"-avP", a.Src, a.Dst}, extraArgs...)
		cmd := execdriver.MustRun(ctx, "rsync", args...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return "", err
		}
		cmd.Stderr = cmd.Stdout
		if err := cmd.Start(); err != nil {
			return "", err
		}
		lastLine := ""
		scanner := bufio.NewScanner(stdout)
		lastUpdate := time.Now()
		for scanner.Scan() {
			line := scanner.Text()
			if line != "" {
				lastLine = line
			}
			if time.Since(lastUpdate) > time.Second {
				if (*notification.Notification)(n) != nil {
					(*notification.Notification)(n).Message = line
					logging.ReportError(ctx, notification.Notify(ctx, n))
				}
				lastUpdate = time.Now()
			}
		}
		err = cmd.Wait()
		return lastLine, err
	}()
	return err
}
