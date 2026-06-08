package apps

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	v1 := "3.4.4"
	v2 := "3.2.7"

	if compareVersions(v1, v2) <= 0 {
		t.Fatalf("expected %s > %s", v1, v2)
	}
}

func TestRsyncPlatformFolder(t *testing.T) {
	tool := &rsyncTool{}
	folder, err := tool.rsyncPlatformFolder()

	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		if err != nil {
			t.Fatalf("expected no error for linux/amd64, got %v", err)
		}
		if folder != "debian-11-x86_64" {
			t.Fatalf("expected debian-11-x86_64, got %s", folder)
		}
	} else if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		if err != nil {
			t.Fatalf("expected no error for darwin/arm64, got %v", err)
		}
		if folder != "macos-12.6-arm64" {
			t.Fatalf("expected macos-12.6-arm64, got %s", folder)
		}
	} else if runtime.GOOS == "windows" {
		if err == nil {
			t.Fatalf("expected error for windows")
		}
	}
}

func TestRsyncArtifactURLs(t *testing.T) {
	tool := &rsyncTool{}

	if runtime.GOOS == "windows" {
		t.Skip("Windows is unsupported")
	}

	folder, err := tool.rsyncPlatformFolder()
	if err != nil {
		t.Skipf("Unsupported platform for test: %v", err)
	}

	artifacts, err := tool.ListArtifacts(context.Background(), "3.2.7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}

	urls := strings.Split(artifacts[0].URL, ",")
	if len(urls) != 1 {
		t.Fatalf("expected 1 URLs since version is explicit, got %d", len(urls))
	}

	expected1 := "https://download.samba.org/pub/rsync/binaries/" + folder + "/rsync-3.2.7.tar.gz"

	if urls[0] != expected1 {
		t.Fatalf("expected %s, got %s", expected1, urls[0])
	}
}
