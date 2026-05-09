package backup

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"workspaced/pkg/configcue"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/notification"
	"workspaced/pkg/logging"
)

type BackupAction interface {
	GetName() string
	GetKind() string
	Run(ctx context.Context, n *notification.Notification) error
}

type backupActionBase struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

func (a backupActionBase) GetName() string { return a.Name }
func (a backupActionBase) GetKind() string { return a.Kind }

type backupConfig struct {
	Backup struct {
		Actions []json.RawMessage `json:"actions"`
	} `json:"backup"`
}

func RunFullBackup(ctx context.Context) error {
	logger := logging.GetLogger(ctx)
	rawCfg, err := configcue.LoadHome()
	if err != nil {
		return err
	}
	var cfg backupConfig
	if err := rawCfg.Decode("", &cfg); err != nil {
		return err
	}

	actions, err := decodeBackupActions(cfg.Backup.Actions)
	if err != nil {
		return err
	}
	if len(actions) == 0 {
		logger.Info("no backup actions configured")
		return nil
	}
	logger.Info("backup started", "actions", len(actions))

	n := &notification.Notification{
		ID:          notification.BackupNotificationID,
		Title:       "Backup em curso",
		Icon:        "drive-harddisk",
		HasProgress: true,
	}
	failures := []string{}

	for i, action := range actions {
		msg := action.GetName()
		if strings.TrimSpace(msg) == "" {
			msg = fmt.Sprintf("Executando ação %d/%d...", i+1, len(actions))
		}
		n.Message = msg
		n.Progress = float64(i+1) / float64(len(actions))
		logging.ReportError(ctx, notification.Notify(ctx, n))
		logger.Info("backup action started", "index", i+1, "total", len(actions), "name", msg, "kind", action.GetKind())
		if err := action.Run(ctx, n); err != nil {
			logger.Error("backup action failed", "name", msg, "kind", action.GetKind(), "error", err)
			failures = append(failures, fmt.Sprintf("%s (%s): %v", msg, action.GetKind(), err))
			continue
		}
		logger.Info("backup action completed", "name", msg, "kind", action.GetKind())
	}

	if len(failures) > 0 {
		logger.Error("backup finished with failures", "count", len(failures))
		n.Title = "Backup finalizado com falhas"
		n.Message = strings.Join(failures, "\n")
		n.Urgency = "critical"
		n.Progress = 1.0
		logging.ReportError(ctx, notification.Notify(ctx, n))
		return fmt.Errorf("backup finished with %d failure(s): %s", len(failures), strings.Join(failures, "; "))
	}

	n.Title = "Backup finalizado"
	n.Progress = 1.0
	logging.ReportError(ctx, notification.Notify(ctx, n))
	logger.Info("backup finished successfully")
	return nil
}

func decodeBackupActions(rawActions []json.RawMessage) ([]BackupAction, error) {
	actions := make([]BackupAction, 0, len(rawActions))
	for _, raw := range rawActions {
		var base backupActionBase
		if err := json.Unmarshal(raw, &base); err != nil {
			return nil, fmt.Errorf("decode backup action envelope: %w", err)
		}
		provider, ok := actionProviders[base.Kind]
		if !ok {
			return nil, fmt.Errorf("unknown backup action kind: %s", base.Kind)
		}
		action, err := provider(raw)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, nil
}

func Rsync(ctx context.Context, src, dst string, n *notification.Notification, extraArgs ...string) (string, error) {
	if strings.TrimSpace(src) == "" || strings.TrimSpace(dst) == "" {
		return "", fmt.Errorf("rsync requires src and dst")
	}
	logging.GetLogger(ctx).Info("rsync sync", "from", src, "to", dst)
	args := append([]string{"-avP", src, dst}, extraArgs...)
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
			if n != nil {
				n.Message = line
				logging.ReportError(ctx, notification.Notify(ctx, n))
			}
			lastUpdate = time.Now()
		}
	}

	err = cmd.Wait()
	return lastLine, err
}
