package taskgroup

import (
	"strings"
	"testing"
)

func TestFormatBarLine(t *testing.T) {
	line := formatBarLine(barEntry{
		title:    "bundle.tar.gz",
		subtitle: "5.0 MiB / 10.0 MiB",
		pool:     Internet,
		percent:  0.5,
	}, 10)
	// ICON BAR title: subtitle — bar is fixed-width after the emoji.
	wantPrefix := "🌐 ["
	if !strings.HasPrefix(line, wantPrefix) {
		t.Fatalf("line = %q, want prefix %q", line, wantPrefix)
	}
	if !strings.Contains(line, "] bundle.tar.gz: 5.0 MiB / 10.0 MiB") {
		t.Fatalf("line = %q, want title: size subtitle after bar", line)
	}
	// No text between icon and bar (old layout put title before the bar).
	if strings.Contains(line, "bundle.tar.gz: [") {
		t.Fatalf("old layout leaked: title before bar in %q", line)
	}
}

func TestFormatBarLinePools(t *testing.T) {
	tests := []struct {
		pool  PoolKind
		emoji string
	}{
		{Control, "🔧"},
		{IO, "💾"},
		{CPU, "🧠"},
		{Internet, "🌐"},
	}
	for _, tt := range tests {
		line := formatBarLine(barEntry{title: "t", subtitle: "s", pool: tt.pool, percent: 0}, 4)
		if !strings.HasPrefix(line, tt.emoji+" ") {
			t.Errorf("pool %v: line = %q, want emoji %s", tt.pool, line, tt.emoji)
		}
	}
}

func TestPlainBar(t *testing.T) {
	if got, want := plainBar(0, 4), "[----]"; got != want {
		t.Errorf("0%% = %q, want %q", got, want)
	}
	if got, want := plainBar(1, 4), "[====]"; got != want {
		t.Errorf("100%% = %q, want %q", got, want)
	}
	if got, want := plainBar(0.5, 4), "[==--]"; got != want {
		t.Errorf("50%% = %q, want %q", got, want)
	}
}

func TestSyncFromSnapshotKeysByID(t *testing.T) {
	m := newBubbleModel(nil)
	// Two tasks with the same description — must remain distinct bars.
	m.syncFromSnapshot([]TaskState{
		{ID: "id-a", Name: "fetch", Pool: Internet, State: Running, Message: "a", Current: 1, Total: 2},
		{ID: "id-b", Name: "fetch", Pool: Internet, State: Running, Message: "b", Current: 2, Total: 2},
	})
	if len(m.bars) != 2 {
		t.Fatalf("bars = %d, want 2 (keyed by ID, not title)", len(m.bars))
	}
	if m.bars["id-a"].title != "fetch" || m.bars["id-a"].subtitle != "a" {
		t.Errorf("id-a = %+v", m.bars["id-a"])
	}
	if m.bars["id-b"].subtitle != "b" {
		t.Errorf("id-b subtitle = %q, want b", m.bars["id-b"].subtitle)
	}
	if len(m.order) != 2 || m.order[0] != "id-a" || m.order[1] != "id-b" {
		t.Fatalf("order = %v, want [id-a id-b]", m.order)
	}

	// Update progress on id-a only; id-b disappears when not running.
	m.syncFromSnapshot([]TaskState{
		{ID: "id-a", Name: "fetch", Pool: Internet, State: Running, Message: "done-ish", Current: 2, Total: 2},
		{ID: "id-b", Name: "fetch", Pool: Internet, State: Done, Message: "b", Current: 2, Total: 2},
	})
	if _, ok := m.bars["id-b"]; ok {
		t.Fatal("finished id-b should be dropped")
	}
	if m.bars["id-a"].subtitle != "done-ish" || m.bars["id-a"].percent != 1 {
		t.Errorf("id-a after update = %+v", m.bars["id-a"])
	}
}

func TestSyncFromSnapshotSkipsIndeterminate(t *testing.T) {
	m := newBubbleModel(nil)
	m.syncFromSnapshot([]TaskState{
		{ID: "x", Name: "wait", Pool: Control, State: Running, Total: -1},
		{ID: "y", Name: "work", Pool: CPU, State: Running, Message: "go", Current: 0, Total: 1},
	})
	if len(m.bars) != 1 {
		t.Fatalf("bars = %d, want only determinate total>0", len(m.bars))
	}
	if _, ok := m.bars["y"]; !ok {
		t.Fatal("missing y")
	}
	if m.bars["y"].subtitle != "go" {
		t.Errorf("subtitle = %q, want go", m.bars["y"].subtitle)
	}
}

func TestSyncFromSnapshotDefaultSubtitle(t *testing.T) {
	m := newBubbleModel(nil)
	m.syncFromSnapshot([]TaskState{
		{ID: "z", Name: "unit", Pool: IO, State: Running, Current: 0, Total: 1},
	})
	if m.bars["z"].subtitle != "running" {
		t.Errorf("subtitle = %q, want running", m.bars["z"].subtitle)
	}
}

func TestViewIncludesLiveRows(t *testing.T) {
	hub := newLiveHub()
	w := newLineWriter(hub, func(string) {})
	if _, err := w.Write([]byte("downloading 10%\rdownloading 50%")); err != nil {
		t.Fatal(err)
	}
	m := newBubbleModel(nil)
	m.live = hub
	content := m.View().Content
	if content != "downloading 50%\n" {
		t.Fatalf("view = %q, want live row only", content)
	}
}

func TestViewLayout(t *testing.T) {
	m := newBubbleModel(nil)
	m.syncFromSnapshot([]TaskState{
		{ID: "1", Name: "bundle.tar.gz", Pool: Internet, State: Running, Message: "5.0 MiB / 10.0 MiB", Current: 50, Total: 100},
		{ID: "2", Name: "build", Pool: CPU, State: Running, Message: "part 1/4", Current: 1, Total: 4},
	})
	content := m.View().Content
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %v (%d), want 2", lines, len(lines))
	}
	// Both lines: ICON BAR title: subtitle
	for i, line := range lines {
		// emoji + space + [bar] + space + title: subtitle
		if !strings.Contains(line, "] ") || !strings.Contains(line, ": ") {
			t.Errorf("line %d = %q, want ICON BAR title: subtitle", i, line)
		}
		// Bar comes before the colon-separated title
		barIdx := strings.Index(line, "[")
		titleIdx := strings.Index(line, "bundle.tar.gz")
		if titleIdx < 0 {
			titleIdx = strings.Index(line, "build")
		}
		if barIdx < 0 || titleIdx < 0 || barIdx > titleIdx {
			t.Errorf("line %d = %q: bar should precede title", i, line)
		}
	}
}
