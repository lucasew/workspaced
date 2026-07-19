package termux

import (
	"context"
	"fmt"
	"github.com/lucasew/workspaced/pkg/driver"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/driver/terminal"
	"strings"
)

func init() {
	driver.Register[terminal.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "terminal_termux" }
func (f *Factory) Name() string { return "Termux" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	return driver.RequireTermux()
}

func (f *Factory) New(ctx context.Context) (terminal.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) Open(ctx context.Context, opts terminal.Options) error {
	if opts.Command == "" {
		// Just bring Termux to front/open new session if configured in app
		return execdriver.MustRun(ctx, "am", "start", "--user", "0", "-n", "com.termux/.app.TermuxActivity").Run()
	}

	fullCmd := opts.Command
	// Resolve full path if it's just a binary name
	if !strings.HasPrefix(fullCmd, "/") {
		if path, err := execdriver.Which(ctx, fullCmd); err == nil {
			fullCmd = path
		}
	}

	if len(opts.Args) > 0 {
		// Proper escaping for the shell string
		var escapedArgs []string
		for _, arg := range opts.Args {
			escapedArgs = append(escapedArgs, fmt.Sprintf("%q", arg))
		}
		fullCmd += " " + strings.Join(escapedArgs, " ")
	}

	return execdriver.MustRun(ctx, "am", "startservice",
		"--user", "0",
		"-n", "com.termux/com.termux.app.TermuxService",
		"-a", "com.termux.service_execute",
		"-e", "com.termux.execute.command", fullCmd,
	).Run()
}
