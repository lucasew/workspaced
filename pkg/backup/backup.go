package backup

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"workspaced/pkg/cmdctx"
	"workspaced/pkg/configcue"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/notification"
	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"
)

var (
	// ErrUnknownBackupActionKind is returned when a backup action has an unrecognized kind.
	ErrUnknownBackupActionKind = errors.New("unknown backup action kind")
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
	rawCfg, err := configcue.LoadHome(ctx)
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
	var failuresMu sync.Mutex
	failures := []string{}

	// Determine pool: rsync/archive → IO, git_repo_sync → Internet.
	poolFor := func(kind string) taskgroup.PoolKind {
		switch kind {
		case "git_repo_sync":
			return taskgroup.Internet
		default:
			return taskgroup.IO
		}
	}

	// Task group must be provided via context from the top-level command.
	parent := taskgroup.MustFromContext(ctx)
	g, ctx := parent.SubGroup(ctx)

	for i, action := range actions {
		idx := i
		act := action
		msg := act.GetName()
		if strings.TrimSpace(msg) == "" {
			msg = fmt.Sprintf("backup-action-%d", idx+1)
		}

		g.Go(fmt.Sprintf("backup:%s", msg), poolFor(act.GetKind()), func(ctx context.Context, s *taskgroup.Status) error {
			logger := logging.GetLogger(ctx)
			s.Update(msg)
			s.Progress(int64(idx), int64(len(actions)))
			logger.Info("backup action started", "index", idx+1, "total", len(actions), "name", msg, "kind", act.GetKind())

			if cmdctx.IsDryRun(ctx) {
				logger.Info("dry-run: skipping", "name", msg)
				logger.Info("backup action completed (dry-run)", "name", msg, "kind", act.GetKind())
				return nil
			}

			n2 := &notification.Notification{
				ID:          notification.BackupNotificationID,
				Title:       "Backup em curso",
				Icon:        "drive-harddisk",
				HasProgress: true,
				Message:     msg,
				Progress:    float64(idx+1) / float64(len(actions)),
			}
			logging.ReportError(ctx, notification.Notify(ctx, n2))

			if err := act.Run(ctx, n2); err != nil {
				logger.Error("backup action failed", "name", msg, "kind", act.GetKind(), "error", err)
				failuresMu.Lock()
				failures = append(failures, fmt.Sprintf("%s (%s): %v", msg, act.GetKind(), err))
				failuresMu.Unlock()
				// Don't return error — let other actions continue.
				return nil
			}
			logger.Info("backup action completed", "name", msg, "kind", act.GetKind())
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
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
			return nil, fmt.Errorf("%w: %s", ErrUnknownBackupActionKind, base.Kind)
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
		return "", ErrRsyncNeedsSrcAndDst
	}
	logger := logging.GetLogger(ctx)
	logger.Info("rsync sync", "from", src, "to", dst)
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
