package dialog

import "testing"

func TestItemLabel(t *testing.T) {
	t.Parallel()
	if got := ItemLabel(Item{Label: "L", Value: "V"}); got != "L" {
		t.Fatalf("ItemLabel with label = %q, want L", got)
	}
	if got := ItemLabel(Item{Value: "V"}); got != "V" {
		t.Fatalf("ItemLabel empty label = %q, want V", got)
	}
}

func TestFormatChoiceLines(t *testing.T) {
	t.Parallel()
	items := []Item{
		{Label: "Alpha", Value: "a", Icon: "icon-a"},
		{Value: "b"},
	}
	got := FormatChoiceLines(items, false)
	want := "Alpha\nb\n"
	if got != want {
		t.Fatalf("without icons:\n got %q\nwant %q", got, want)
	}
	got = FormatChoiceLines(items, true)
	want = "Alpha\x00icon\x1ficon-a\nb\n"
	if got != want {
		t.Fatalf("with icons:\n got %q\nwant %q", got, want)
	}
}

func TestMatchSelected(t *testing.T) {
	t.Parallel()
	items := []Item{
		{Label: "Alpha", Value: "a"},
		{Value: "b"},
	}

	if got := MatchSelected(items, "   "); got != nil {
		t.Fatalf("empty selected = %+v, want nil", got)
	}

	got := MatchSelected(items, "Alpha")
	if got == nil || got.Value != "a" {
		t.Fatalf("match label = %+v, want Value=a", got)
	}
	got = MatchSelected(items, "b")
	if got == nil || got.Value != "b" {
		t.Fatalf("match value-as-label = %+v, want Value=b", got)
	}

	got = MatchSelected(items, "other")
	if got == nil || got.Label != "other" || got.Value != "other" {
		t.Fatalf("unknown selected = %+v, want synthetic other", got)
	}
}
