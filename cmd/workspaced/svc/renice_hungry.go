package svc

import (
	"context"
	"strings"
	"time"

	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/logging"
)

type ReniceHungry struct{}

func (ReniceHungry) Description() string {
	return "Lowers the priority of the most cpu hungry process periodically"
}

func (*ReniceHungry) Run(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	logger := logging.GetLogger(ctx)
	logger.Info("renice-hungry started")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			pid, cmdline, err := getHungryPID(ctx)
			if err != nil {
				logger.Error("failed to get hungry PID", "error", err)
				continue
			}
			if pid == "" {
				continue
			}

			logger.Info("renicing process", "pid", pid, "cmd", cmdline)
			if err := execdriver.MustRun(ctx, "renice", "7", pid).Run(); err != nil {
				logger.Error("failed to renice process", "pid", pid, "cmd", cmdline, "error", err)
				continue
			}
		}
	}
}

func getHungryPID(ctx context.Context) (string, string, error) {
	// ps -eo pid,args --sort=-%cpu | head -n2 | tail -n 1
	out, err := execdriver.MustRun(ctx, "ps", "-eo", "pid,args", "--sort=-%cpu").Output()
	if err != nil {
		return "", "", err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return "", "", nil
	}

	// First line is header, second line is top process
	line := strings.TrimSpace(lines[1])
	pid, rest, ok := strings.Cut(line, " ")
	cmdline := ""
	if ok {
		cmdline = strings.TrimSpace(rest)
	}
	return pid, cmdline, nil
}
