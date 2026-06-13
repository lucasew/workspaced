package deployer

import "testing"

func TestSortActions(t *testing.T) {
	actions := []Action{
		{Type: ActionCreate, Target: "/b"},
		{Type: ActionDelete, Target: "/a"},
		{Type: ActionNoop, Target: "/c"},
		{Type: ActionUpdate, Target: "/d"},
		{Type: ActionCreate, Target: "/a"},
		{Type: ActionDelete, Target: "/b"},
	}

	sorted := SortActions(actions)

	// Verify order: Delete < Update < Create < Noop, then by Target
	expected := []struct {
		actionType ActionType
		target     string
	}{
		{ActionDelete, "/a"},
		{ActionDelete, "/b"},
		{ActionUpdate, "/d"},
		{ActionCreate, "/a"},
		{ActionCreate, "/b"},
		{ActionNoop, "/c"},
	}

	if len(sorted) != len(expected) {
		t.Fatalf("got %d actions, want %d", len(sorted), len(expected))
	}

	for i, want := range expected {
		if sorted[i].Type != want.actionType || sorted[i].Target != want.target {
			t.Errorf("sorted[%d] = {%s, %s}, want {%s, %s}",
				i, sorted[i].Type, sorted[i].Target, want.actionType, want.target)
		}
	}

	// Verify original slice is not modified
	if actions[0].Type != ActionCreate || actions[0].Target != "/b" {
		t.Error("SortActions mutated the original slice")
	}
}

func TestActionTypeString(t *testing.T) {
	tests := []struct {
		action ActionType
		want   string
	}{
		{ActionCreate, "+"},
		{ActionUpdate, "*"},
		{ActionDelete, "-"},
		{ActionNoop, " "},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.action.String()
			if got != tt.want {
				t.Errorf("ActionType(%d).String() = %q, want %q", tt.action, got, tt.want)
			}
		})
	}
}
