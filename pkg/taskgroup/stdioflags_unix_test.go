//go:build unix

package taskgroup

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func TestIsCharDevicePipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})
	if isCharDevice(w) {
		t.Fatal("pipe must not be a char device")
	}
}

func TestIsInteractiveTerminalEnvGuards(t *testing.T) {
	t.Setenv("WORKSPACED_FORCE_TUI", "")
	for _, env := range []struct{ key, val string }{
		{"TERM", "dumb"},
		{"NO_COLOR", "1"},
		{"CI", "1"},
	} {
		t.Run(env.key, func(t *testing.T) {
			t.Setenv("TERM", "xterm")
			t.Setenv("NO_COLOR", "")
			t.Setenv("CI", "")
			t.Setenv(env.key, env.val)
			if isInteractiveTerminal() {
				t.Fatalf("%s=%s must disable the progress TUI", env.key, env.val)
			}
		})
	}
}

func TestIsInteractiveTerminalPipedStdout(t *testing.T) {
	t.Setenv("WORKSPACED_FORCE_TUI", "")
	t.Setenv("TERM", "xterm")
	t.Setenv("NO_COLOR", "")
	t.Setenv("CI", "")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})
	old := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	if isInteractiveTerminal() {
		t.Fatal("piped stdout must not start the progress TUI")
	}
}

func TestStdioFlagsRestoreClearsNonblock(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})
	fd := int(w.Fd())
	orig, err := fcntlGet(fd)
	if err != nil {
		t.Fatal(err)
	}
	if err := fcntlSet(fd, orig|oNonblock); err != nil {
		t.Fatal(err)
	}
	got, err := fcntlGet(fd)
	if err != nil {
		t.Fatal(err)
	}
	if got&oNonblock == 0 {
		t.Fatal("expected O_NONBLOCK after set")
	}
	if err := fcntlSet(fd, orig); err != nil {
		t.Fatal(err)
	}
	got, err = fcntlGet(fd)
	if err != nil {
		t.Fatal(err)
	}
	if got&oNonblock != orig&oNonblock {
		t.Fatalf("restore: flags=%#x want nonblock bit %#x", got, orig&oNonblock)
	}
}

func TestSnapshotStdioFlagsRoundTrip(t *testing.T) {
	snap := snapshotStdioFlags()
	if len(snap.fds) == 0 {
		t.Fatal("expected to snapshot at least one stdio fd")
	}
	// Flip stdout nonblock, then restore the snapshot.
	out := unix.Stdout
	before, err := fcntlGet(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := fcntlSet(out, before|oNonblock); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fcntlSet(out, before) })
	snap.restore()
	after, err := fcntlGet(out)
	if err != nil {
		t.Fatal(err)
	}
	if after&oNonblock != before&oNonblock {
		t.Fatalf("snapshot restore left O_NONBLOCK: before=%#x after=%#x", before, after)
	}
}
