package backup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"workspaced/internal/cmdctx"
	"workspaced/internal/configcue"
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

// actionOutcome is the map-step result for one backup action.
type actionOutcome struct {
	Name string
	Kind string
	Err  error
}

func (o actionOutcome) String() string {
	return fmt.Sprintf("%s (%s): %v", o.Name, o.Kind, o.Err)
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

	// Determine pool: rsync/archive → IO, git_repo_sync → Internet.
	poolFor := func(kind string) taskgroup.PoolKind {
		switch kind {
		case "git_repo_sync":
			return taskgroup.Internet
		case "rsync":
			return taskgroup.Control // rsync driver manages its own IO tasks
		default:
			return taskgroup.IO
		}
	}

	type actionItem struct {
		Idx    int
		Action BackupAction
	}
	actionLabel := func(idx int, act BackupAction) (name, kind string) {
		name = act.GetName()
		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("backup-action-%d", idx+1)
		}
		return name, act.GetKind()
	}

	items := make([]actionItem, len(actions))
	for i, a := range actions {
		items[i] = actionItem{Idx: i, Action: a}
	}

	// Map each action to an outcome; reduce collects Err != nil.
	outcomes, mapErr := taskgroup.Map[actionItem, actionOutcome]{
		Name:  "backup",
		Items: items,
		Pool: func(item actionItem) taskgroup.PoolKind {
			return poolFor(item.Action.GetKind())
		},
		TaskName: func(_ int, item actionItem) string {
			name, _ := actionLabel(item.Idx, item.Action)
			return "backup:" + name
		},
		Fn: func(ctx context.Context, s *taskgroup.Status, item actionItem) (actionOutcome, error) {
			name, kind := actionLabel(item.Idx, item.Action)
			out := actionOutcome{Name: name, Kind: kind}

			logger := logging.GetLogger(ctx)
			s.Update(name)
			logger.Info("backup action started", "index", item.Idx+1, "total", len(actions), "name", name, "kind", kind)

			if cmdctx.IsDryRun(ctx) {
				logger.Info("dry-run: skipping", "name", name)
				logger.Info("backup action completed (dry-run)", "name", name, "kind", kind)
				return out, nil
			}

			n2 := &notification.Notification{
				ID:          notification.BackupNotificationID,
				Title:       "Backup em curso",
				Icon:        "drive-harddisk",
				HasProgress: true,
				Message:     name,
				Progress:    float64(item.Idx+1) / float64(len(actions)),
			}
			logging.ReportError(ctx, notification.Notify(ctx, n2))

			if err := item.Action.Run(ctx, n2); err != nil {
				logger.Error("backup action failed", "name", name, "kind", kind, "error", err)
				out.Err = err
				return out, nil
			}

			logger.Info("backup action completed", "name", name, "kind", kind)
			return out, nil
		},
	}.Run(ctx)
	if mapErr != nil {
		return mapErr
	}

	var failures []string
	for _, o := range outcomes {
		if o.Err != nil {
			failures = append(failures, o.String())
		}
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
		decode, ok := actionDecoders[base.Kind]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownBackupActionKind, base.Kind)
		}
		action, err := decode(raw)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, nil
}
