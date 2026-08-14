package taskgroup

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestFormatBarLineWide(t *testing.T) {
	line := formatBarLine(barEntry{
		title:    "bundle.tar.gz",
		subtitle: "5.0 MiB / 10.0 MiB",
		pool:     Internet,
		percent:  0.5,
	}, 80)
	if !strings.HasPrefix(line, "🌐 bundle.tar.gz: 5.0 MiB / 10.0 MiB [") {
		t.Fatalf("wide = %q, want emoji message [bar]", line)
	}
	if !strings.HasSuffix(line, "]") {
		t.Fatalf("wide = %q, want trailing ]", line)
	}
	if cellWidth(line) != 80 {
		t.Fatalf("wide width = %d, want 80 (%q)", cellWidth(line), line)
	}
}

func TestFormatBarLineNarrow(t *testing.T) {
	line := formatBarLine(barEntry{
		title:    "bundle.tar.gz",
		subtitle: "5.0 MiB / 10.0 MiB",
		pool:     Internet,
		percent:  0.5,
	}, 40)
	if !strings.HasPrefix(line, "🌐  50.0% ") {
		t.Fatalf("narrow = %q, want emoji percent message", line)
	}
	if strings.Contains(line, "[") {
		t.Fatalf("narrow kept a bar: %q", line)
	}
	if !strings.Contains(line, "bundle.tar.gz") {
		t.Fatalf("narrow missing message: %q", line)
	}
	if cellWidth(line) > 40 {
		t.Fatalf("narrow width = %d > 40 (%q)", cellWidth(line), line)
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
		line := formatBarLine(barEntry{title: "t", subtitle: "s", pool: tt.pool, percent: 0}, 80)
		if !strings.HasPrefix(line, tt.emoji+" ") {
			t.Errorf("pool %v: line = %q, want emoji %s", tt.pool, line, tt.emoji)
		}
	}
}

func TestFormatPercent(t *testing.T) {
	if got, want := formatPercent(0.45231), " 45.2%"; got != want {
		t.Fatalf("formatPercent(0.45231) = %q, want %q", got, want)
	}
	if got, want := formatPercent(1), "100.0%"; got != want {
		t.Fatalf("formatPercent(1) = %q, want %q", got, want)
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
	m.width = 80
	m.syncFromSnapshot([]TaskState{
		{ID: "1", Name: "bundle.tar.gz", Pool: Internet, State: Running, Message: "5.0 MiB / 10.0 MiB", Current: 50, Total: 100},
		{ID: "2", Name: "build", Pool: CPU, State: Running, Message: "part 1/4", Current: 1, Total: 4},
	})
	content := m.View().Content
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %v (%d), want 2", lines, len(lines))
	}
	for i, line := range lines {
		if !strings.Contains(line, ": ") || !strings.Contains(line, " [") {
			t.Errorf("line %d = %q, want emoji message [bar]", i, line)
		}
		barIdx := strings.Index(line, "[")
		titleIdx := strings.Index(line, "bundle.tar.gz")
		if titleIdx < 0 {
			titleIdx = strings.Index(line, "build")
		}
		if barIdx < 0 || titleIdx < 0 || titleIdx > barIdx {
			t.Errorf("line %d = %q: message should precede bar", i, line)
		}
	}
}

func TestViewResizeSwitchesLayout(t *testing.T) {
	m := newBubbleModel(nil)
	m.syncFromSnapshot([]TaskState{
		{ID: "1", Name: "build", Pool: CPU, State: Running, Message: "part 1/4", Current: 1, Total: 4},
	})

	next, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	m = next.(bubbleModel)
	narrow := strings.TrimSuffix(m.View().Content, "\n")
	if !strings.Contains(narrow, "%") || strings.Contains(narrow, "[") {
		t.Fatalf("width 40 = %q, want percent layout", narrow)
	}

	next, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(bubbleModel)
	wide := strings.TrimSuffix(m.View().Content, "\n")
	if !strings.Contains(wide, "[") || strings.Contains(wide, "%") {
		t.Fatalf("width 80 = %q, want expanding bar", wide)
	}
	if cellWidth(wide) != 80 {
		t.Fatalf("width 80 line is %d cells: %q", cellWidth(wide), wide)
	}
}
