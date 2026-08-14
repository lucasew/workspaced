package taskgroup

import (
	"strings"
	"testing"
)

func feed(t *testing.T, chunks ...string) (committed []string, current string) {
	t.Helper()
	var f lineFilter
	for _, c := range chunks {
		committed = append(committed, f.write([]byte(c))...)
	}
	return committed, f.current()
}

func TestLineFilterPlain(t *testing.T) {
	got, cur := feed(t, "hello\nworld")
	if strings.Join(got, "|") != "hello" {
		t.Fatalf("committed = %q, want [hello]", got)
	}
	if cur != "world" {
		t.Fatalf("current = %q, want world", cur)
	}
}

func TestLineFilterCRProgress(t *testing.T) {
	got, cur := feed(t, "downloading 10%\rdownloading 50%\rdownloading 100%\ndone\n")
	if strings.Join(got, "|") != "downloading 100%|done" {
		t.Fatalf("committed = %q", got)
	}
	if cur != "" {
		t.Fatalf("current = %q, want empty", cur)
	}
}

func TestLineFilterCRShorterReplacesLine(t *testing.T) {
	// Demo-shaped: last "N/N" frame is longer than "done".
	got, cur := feed(t, "alpha 24/24\ralpha done\n", "beta 16/16\rbeta done\n", "gamma 20/20\rgamma done\n")
	if strings.Join(got, "|") != "alpha done|beta done|gamma done" {
		t.Fatalf("committed = %q", got)
	}
	if cur != "" {
		t.Fatalf("current = %q", cur)
	}
}

func TestLineFilterCRLFCommitsPrior(t *testing.T) {
	got, cur := feed(t, "hello\r\nworld\r\n")
	if strings.Join(got, "|") != "hello|world" {
		t.Fatalf("committed = %q, want [hello world]", got)
	}
	if cur != "" {
		t.Fatalf("current = %q", cur)
	}
}

func TestLineFilterCRReplaceCurrent(t *testing.T) {
	_, cur := feed(t, "hello\rX")
	if cur != "X" {
		t.Fatalf("current = %q, want X (not Xello)", cur)
	}
}

func TestLineFilterCRAcrossChunks(t *testing.T) {
	got, cur := feed(t, "10%\r", "50%\r", "100%\n")
	if strings.Join(got, "|") != "100%" {
		t.Fatalf("committed = %q, want [100%%]", got)
	}
	if cur != "" {
		t.Fatalf("current = %q", cur)
	}
}

func TestLineFilterStripSGR(t *testing.T) {
	got, cur := feed(t, "\x1b[31merror\x1b[0m: boom\n")
	if strings.Join(got, "|") != "error: boom" || cur != "" {
		t.Fatalf("committed=%q current=%q", got, cur)
	}
}

func TestLineFilterEraseLineSpinner(t *testing.T) {
	raw := "\x1b[2K\x1b[37m⠋\x1b[0m Resolving...\r" +
		"\x1b[2K\x1b[37m⠙\x1b[0m Resolving...\r" +
		"\x1b[2KResolved 1 package\n"
	got, cur := feed(t, raw)
	if strings.Join(got, "|") != "Resolved 1 package" {
		t.Fatalf("committed = %q", got)
	}
	if strings.Contains(strings.Join(got, ""), "Resolving") || cur != "" {
		t.Fatalf("spinner leaked: committed=%q current=%q", got, cur)
	}
}

func TestLineFilterIgnoresCursorUp(t *testing.T) {
	// Cursor-up must not un-commit or overwrite the previous line.
	got, cur := feed(t, "hello\n\x1b[1Axxx")
	if strings.Join(got, "|") != "hello" {
		t.Fatalf("committed = %q, want [hello]", got)
	}
	if cur != "xxx" {
		t.Fatalf("current = %q, want xxx (cursor-up ignored)", cur)
	}
}

func TestLineFilterIgnoresEraseDisplay(t *testing.T) {
	got, cur := feed(t, "keep\n\x1b[2Jgone")
	if strings.Join(got, "|") != "keep" {
		t.Fatalf("committed = %q, want [keep]", got)
	}
	if cur != "gone" {
		t.Fatalf("current = %q, want gone", cur)
	}
}

func TestLineFilterOSCHyperlink(t *testing.T) {
	got, _ := feed(t, "\x1b]8;;https://example.com\x07click\x1b]8;;\x07 me\n")
	if strings.Join(got, "|") != "click me" {
		t.Fatalf("committed = %q", got)
	}
}

func TestLineFilterIncompleteESCHeld(t *testing.T) {
	var f lineFilter
	if got := f.write([]byte("hi\x1b")); len(got) != 0 || f.current() != "hi" {
		t.Fatalf("partial ESC: committed=%q current=%q", got, f.current())
	}
	got := f.write([]byte("[31mX\x1b[0m\n"))
	if strings.Join(got, "|") != "hiX" {
		t.Fatalf("after completing ESC: committed=%q current=%q", got, f.current())
	}
}

func TestLineFilterTakeLeftover(t *testing.T) {
	var f lineFilter
	f.write([]byte("partial"))
	if got := f.take(); got != "partial" {
		t.Fatalf("take = %q", got)
	}
	if f.current() != "" {
		t.Fatalf("current after take = %q", f.current())
	}
}

func TestLineFilterEmptyNewlineSkipped(t *testing.T) {
	got, _ := feed(t, "\n\nfoo\n")
	if strings.Join(got, "|") != "foo" {
		t.Fatalf("committed = %q, want [foo]", got)
	}
}

func TestLineFilterBackspace(t *testing.T) {
	got, _ := feed(t, "ab\b c\n")
	if strings.Join(got, "|") != "a c" {
		t.Fatalf("committed = %q, want [a c]", got)
	}
}
