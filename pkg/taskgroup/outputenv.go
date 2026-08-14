package taskgroup

import (
	"io"
	"log"
	"log/slog"
	"os"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/lucasew/workspaced/pkg/logging"
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
// a lineWriter (via a pipe-backed teaWriter) so CR / in-line CSI rewrite one
// live row and newlines commit through prog.Printf. No Send — Send after Quit
// can block the pipe copy and stall restore.
func newOutputEnv(prog *tea.Program, hub *liveHub) *outputEnv {
	e := &outputEnv{
		oldStderr: os.Stderr,
		oldSlog:   slog.Default(),
		oldLogOut: log.Default().Writer(),
	}

	tw := &teaWriter{raw: newLineWriter(hub, func(s string) {
		prog.Printf("%s", s)
	})}
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

func (e *outputEnv) restore() error {
	if e == nil {
		return nil
	}
	var restoreErr error
	e.restoreOnce.Do(func() {
		slog.SetDefault(e.oldSlog)
		log.SetOutput(e.oldLogOut)
		os.Stderr = e.oldStderr
		if e.tw != nil {
			restoreErr = e.tw.close()
			e.tw = nil
		}
	})
	return restoreErr
}
