package taskgroup

import (
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"workspaced/pkg/logging"
)

// outputEnv holds process-wide output redirections while the session UI is
// active. restore is safe to call multiple times (sync.Once).
type outputEnv struct {
	oldStderr *os.File
	oldSlog   *slog.Logger
	oldLogOut io.Writer
	tw        *teaWriter

	restoreOnce sync.Once
}

// newOutputEnv patches os.Stderr, slog.Default, and log.Default to converge on
// a teaWriter that prints complete lines via prog.Printf (no Send — Send after
// Quit can block the pipe copy and stall restore).
func newOutputEnv(prog *tea.Program) *outputEnv {
	e := &outputEnv{
		oldStderr: os.Stderr,
		oldSlog:   slog.Default(),
		oldLogOut: log.Default().Writer(),
	}

	tw := &teaWriter{print: func(s string) {
		s = strings.ReplaceAll(s, "\r", "")
		s = strings.ReplaceAll(s, "\n", "")
		prog.Printf("%s", s)
	}}
	e.tw = tw

	if f, err := tw.File(); err == nil {
		os.Stderr = f
	}

	h := logging.NewPlainHandler(tw, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(h))
	log.SetOutput(tw)
	return e
}

func (e *outputEnv) restore() {
	if e == nil {
		return
	}
	e.restoreOnce.Do(func() {
		slog.SetDefault(e.oldSlog)
		log.SetOutput(e.oldLogOut)
		os.Stderr = e.oldStderr
		if e.tw != nil {
			e.tw.close()
			e.tw = nil
		}
	})
}
