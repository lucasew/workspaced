package cmdctx

import (
	"testing"
)

func TestEnvNoCache(t *testing.T) {
	t.Setenv("WORKSPACED_NO_CACHE", "")
	if EnvNoCache() {
		t.Fatal("empty should be off")
	}
	for _, off := range []string{"0", "false", "FALSE", "no", "off"} {
		t.Setenv("WORKSPACED_NO_CACHE", off)
		if EnvNoCache() {
			t.Fatalf("%q should be off", off)
		}
	}
	for _, on := range []string{"1", "true", "yes", "anything"} {
		t.Setenv("WORKSPACED_NO_CACHE", on)
		if !EnvNoCache() {
			t.Fatalf("%q should be on", on)
		}
	}
}

func TestWithNoCache(t *testing.T) {
	ctx := t.Context()
	if IsNoCache(ctx) {
		t.Fatal("default off")
	}
	ctx = WithNoCache(ctx, true)
	if !IsNoCache(ctx) {
		t.Fatal("expected on")
	}
	ctx = WithNoCache(ctx, false)
	if IsNoCache(ctx) {
		t.Fatal("expected off")
	}
}
