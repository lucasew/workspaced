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

// githubTokenStop is planted on the env of the `gh auth token` child so a
// workspaced gh shim that re-enters Token does not call gh again (forkbomb).
// It is never used as a Bearer token.
const githubTokenStop = "STOP"

var (
	tokenOnce sync.Once
	token     string
)

func Token(ctx context.Context) string {
	tokenOnce.Do(func() {
		token = resolveToken(ctx)
	})
	return token
}

func resolveToken(ctx context.Context) string {
	logger := logging.GetLogger(ctx)
	if envToken := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); envToken != "" {
		if envToken == githubTokenStop {
			logger.Info("github token unavailable: re-entered during gh token probe, using anonymous requests")
			return ""
		}
		logger.Info("using github token from environment")
		return envToken
	}

	if !execdriver.IsBinaryAvailable(ctx, "gh") {
		logger.Warn("github token unavailable: gh not found in PATH, using anonymous requests")
		return ""
	}

	ghCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd, err := execdriver.Run(ghCtx, "gh", "auth", "token")
	if err != nil {
		logger.Warn("github token unavailable: failed to create gh auth token command, using anonymous requests", "error", err)
		return ""
	}
	// GITHUB_TOKEN=STOP: nested workspaced (e.g. lazy gh shim) must not re-enter this path.
	cmd.Env = append(os.Environ(),
		"GH_PROMPT_DISABLED=1",
		"GIT_TERMINAL_PROMPT=0",
		"GITHUB_TOKEN="+githubTokenStop,
	)
	out, err := cmd.Output()
	if err != nil {
		logger.Warn("github token unavailable: gh auth token failed, using anonymous requests", "error", err)
		return ""
	}
	got := strings.TrimSpace(string(out))
	if got == "" {
		logger.Warn("github token unavailable: gh auth token returned empty output, using anonymous requests")
		return ""
	}
	// Never treat the probe sentinel as a real token if gh echoes env oddly.
	if got == githubTokenStop {
		logger.Warn("github token unavailable: gh auth token returned probe sentinel, using anonymous requests")
		return ""
	}
	logger.Info("using github token from gh auth token")
	return got
}

func ApplyAuth(ctx context.Context, req *http.Request) {
	if req == nil {
		return
	}
	if token := Token(ctx); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}
