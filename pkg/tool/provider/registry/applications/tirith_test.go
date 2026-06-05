package apps

import (
	"context"
	"slices"
	"testing"
)

func TestTirithListVersionsSkipsThreatDatabaseReleases(t *testing.T) {
	t.Parallel()

	tool := tirithTool{inner: stubTool{versions: []string{
		"threatdb-26940486720-1",
		"threatdb-26874685865-1",
		"v0.3.1",
		"threatdb-25594072496-1",
		"v0.3.0",
		"v0.2.12",
	}}}

	got, err := tool.ListVersions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"v0.3.1", "v0.3.0", "v0.2.12"}

	if !slices.Equal(got, want) {
		t.Fatalf("ListVersions() = %v, want %v", got, want)
	}
}
