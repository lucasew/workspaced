package apps

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestClaudeCodeListVersionsResolvesChannelsToConcreteVersions(t *testing.T) {
	t.Parallel()

	var tool *claudeCodeTool
	tool = &claudeCodeTool{
		baseURL: "https://downloads.claude.ai/claude-code-releases",
		fetchURL: func(_ context.Context, url string) ([]byte, error) {
			switch url {
			case "https://downloads.claude.ai/claude-code-releases/latest":
				return []byte("2.1.162\n"), nil
			case "https://downloads.claude.ai/claude-code-releases/stable":
				return []byte("2.1.152\n"), nil
			default:
				return nil, fmt.Errorf("unexpected url %q", url)
			}
		},
	}

	got, err := tool.ListVersions(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"2.1.162", "2.1.152"}
	if !slices.Equal(got, want) {
		t.Fatalf("ListVersions() = %v, want %v", got, want)
	}
}

func TestClaudeCodeListArtifactsUsesManifestPlatformBinary(t *testing.T) {
	t.Parallel()

	var tool *claudeCodeTool
	tool = &claudeCodeTool{
		baseURL: "https://downloads.claude.ai/claude-code-releases",
		fetchURL: func(_ context.Context, url string) ([]byte, error) {
			if url != "https://downloads.claude.ai/claude-code-releases/2.1.89/manifest.json" {
				return nil, fmt.Errorf("unexpected url %q", url)
			}

			platform := tool.currentPlatform()
			binary := "claude"
			if strings.HasPrefix(platform, "win32") {
				binary = "claude.exe"
			}

			return []byte(fmt.Sprintf(`{
  "version": "2.1.89",
  "platforms": {
    "%s": {
      "binary": "%s",
      "checksum": "903cb3c96b314d86856632c8702f5cdf971b804d0b19ef87446573bcd1d7df1c",
      "size": 228473472
    }
  }
}`, platform, binary)), nil
		},
	}

	artifacts, err := tool.ListArtifacts(context.Background(), "2.1.89")
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("artifact count = %d, want 1", len(artifacts))
	}

	platform := tool.currentPlatform()
	binary := "claude"
	if strings.HasPrefix(platform, "win32") {
		binary = "claude.exe"
	}
	wantURL := fmt.Sprintf("https://downloads.claude.ai/claude-code-releases/2.1.89/%s/%s", platform, binary)

	if artifacts[0].URL != wantURL {
		t.Fatalf("artifact URL = %q, want %q", artifacts[0].URL, wantURL)
	}
	if artifacts[0].Hash != "sha256:903cb3c96b314d86856632c8702f5cdf971b804d0b19ef87446573bcd1d7df1c" {
		t.Fatalf("artifact hash = %q", artifacts[0].Hash)
	}
	if artifacts[0].Size != 228473472 {
		t.Fatalf("artifact size = %d, want %d", artifacts[0].Size, 228473472)
	}
}
