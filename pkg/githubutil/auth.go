package githubutil

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	execdriver "workspaced/pkg/driver/exec"
)

var (
	tokenOnce sync.Once
	token     string
)

func Token(ctx context.Context) string {
	tokenOnce.Do(func() {
		if envToken := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); envToken != "" {
			slog.Info("using github token from environment")
			token = envToken
			return
		}

		if !execdriver.IsBinaryAvailable(ctx, "gh") {
			slog.Warn("github token unavailable: gh not found in PATH, using anonymous requests")
			return
		}

		cmd, err := execdriver.Run(ctx, "gh", "auth", "token")
		if err != nil {
			slog.Warn("github token unavailable: failed to create gh auth token command, using anonymous requests", "error", err)
			return
		}
		out, err := cmd.Output()
		if err != nil {
			slog.Warn("github token unavailable: gh auth token failed, using anonymous requests", "error", err)
			return
		}
		token = strings.TrimSpace(string(out))
		if token == "" {
			slog.Warn("github token unavailable: gh auth token returned empty output, using anonymous requests")
			return
		}
		slog.Info("using github token from gh auth token")
	})
	return token
}

func ApplyAuth(ctx context.Context, req *http.Request) {
	if req == nil {
		return
	}
	if token := Token(ctx); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}
