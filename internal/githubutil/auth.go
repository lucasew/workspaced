package githubutil

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/logging"
)

var (
	tokenOnce sync.Once
	token     string
)

func Token(ctx context.Context) string {
	tokenOnce.Do(func() {
		logger := logging.GetLogger(ctx)
		if envToken := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); envToken != "" {
			logger.Info("using github token from environment")
			token = envToken
			return
		}

		if !execdriver.IsBinaryAvailable(ctx, "gh") {
			logger.Warn("github token unavailable: gh not found in PATH, using anonymous requests")
			return
		}

		ghCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		cmd, err := execdriver.Run(ghCtx, "gh", "auth", "token")
		if err != nil {
			logger.Warn("github token unavailable: failed to create gh auth token command, using anonymous requests", "error", err)
			return
		}
		cmd.Env = append(os.Environ(),
			"GH_PROMPT_DISABLED=1",
			"GIT_TERMINAL_PROMPT=0",
		)
		out, err := cmd.Output()
		if err != nil {
			logger.Warn("github token unavailable: gh auth token failed, using anonymous requests", "error", err)
			return
		}
		token = strings.TrimSpace(string(out))
		if token == "" {
			logger.Warn("github token unavailable: gh auth token returned empty output, using anonymous requests")
			return
		}
		logger.Info("using github token from gh auth token")
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
