package daemon

import (
	"net/http"
	"testing"
)

func TestCheckOriginAllowsLocalClients(t *testing.T) {
	for _, origin := range []string{"", "null"} {
		r := &http.Request{Header: http.Header{}}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if !upgrader.CheckOrigin(r) {
			t.Fatalf("CheckOrigin(%q) = false, want true", origin)
		}
	}
}

func TestCheckOriginRejectsForeignBrowserOrigin(t *testing.T) {
	r := &http.Request{Header: http.Header{"Origin": []string{"https://evil.example"}}}
	if upgrader.CheckOrigin(r) {
		t.Fatal("CheckOrigin(evil) = true, want false")
	}
}
