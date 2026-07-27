package githubutil

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/logging"
)

// githubTokenStop is a legacy re-entry sentinel formerly planted as
// GITHUB_TOKEN=STOP on the `gh auth token` child. Still recognized so nested
// workspaced processes with that env never treat STOP as a Bearer token.
// Prefer githubTokenProbeEnv for new probes: setting GITHUB_TOKEN=STOP poisons
// a real gh CLI (it echoes STOP as the token instead of keyring credentials).
const githubTokenStop = "STOP"

// githubTokenProbeEnv is planted on the `gh auth token` child. Nested
// workspaced (PATH shim → open lazy → ensure) sees it and skips Token's gh
// probe, breaking the forkbomb without overriding gh's own auth sources.
const (
	githubTokenProbeEnv = "WORKSPACED_GITHUB_TOKEN_PROBE"
	githubTokenProbeVal = "1"
)

// GHLocator returns an absolute path to a real gh binary (typically via
// tool ensure of github:cli/cli). Used when `gh` is not on PATH. Must not
// import githubutil call sites that would cycle; register from internal/tool.
type GHLocator func(ctx context.Context) (string, error)

var (
	tokenOnce      sync.Once
	token          string
	resolvingToken atomic.Bool

	locateMu sync.RWMutex
	locateGH GHLocator

	errGHNotFound = errors.New("gh not found on PATH and tool locator unavailable or failed")
)

// SetGHLocator registers a fallback that ensures/resolves a real gh binary
// when PATH does not have one. Pass nil to clear. Safe for concurrent use.
func SetGHLocator(fn GHLocator) {
	locateMu.Lock()
	defer locateMu.Unlock()
	locateGH = fn
}

func getGHLocator() GHLocator {
	locateMu.RLock()
	defer locateMu.RUnlock()
	return locateGH
}

func Token(ctx context.Context) string {
	// Nested process mid-probe, or same-process re-entry while ensuring gh.
	if probeEnvActive() || resolvingToken.Load() {
		return ""
	}
	tokenOnce.Do(func() {
		resolvingToken.Store(true)
		defer resolvingToken.Store(false)
		token = resolveToken(ctx)
	})
	return token
}

func probeEnvActive() bool {
	return strings.TrimSpace(os.Getenv(githubTokenProbeEnv)) == githubTokenProbeVal
}

func resolveToken(ctx context.Context) string {
	logger := logging.GetLogger(ctx)
	if envToken := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); envToken != "" {
		if envToken == githubTokenStop {
			logger.Info("github token unavailable: re-entered during gh token probe (legacy STOP), using anonymous requests")
			return ""
		}
		logger.Info("using github token from environment")
		return envToken
	}

	ghBin, err := resolveGHBinary(ctx)
	if err != nil || ghBin == "" {
		logger.Warn("github token unavailable: gh not available, using anonymous requests", "error", err)
		return ""
	}

	ghCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd, err := execdriver.Run(ghCtx, ghBin, "auth", "token")
	if err != nil {
		logger.Warn("github token unavailable: failed to create gh auth token command, using anonymous requests", "error", err)
		return ""
	}
	// Probe env only — do not set GITHUB_TOKEN=STOP; that makes real gh report
	// STOP as the token and breaks legitimate keyring/login credentials.
	cmd.Env = append(os.Environ(),
		"GH_PROMPT_DISABLED=1",
		"GIT_TERMINAL_PROMPT=0",
		githubTokenProbeEnv+"="+githubTokenProbeVal,
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
	if got == githubTokenStop {
		logger.Warn("github token unavailable: gh auth token returned probe sentinel, using anonymous requests")
		return ""
	}
	logger.Info("using github token from gh auth token", "gh", ghBin)
	return got
}

// resolveGHBinary finds a gh executable: PATH first, then the optional tool
// locator (ensure github:cli/cli) when PATH has no gh.
func resolveGHBinary(ctx context.Context) (string, error) {
	logger := logging.GetLogger(ctx)
	if execdriver.IsBinaryAvailable(ctx, "gh") {
		return "gh", nil
	}
	fn := getGHLocator()
	if fn == nil {
		return "", errGHNotFound
	}
	logger.Info("gh not on PATH; ensuring via tool locator")
	path, err := fn(ctx)
	if err != nil {
		return "", err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errGHNotFound
	}
	return path, nil
}

func ApplyAuth(ctx context.Context, req *http.Request) {
	if req == nil {
		return
	}
	if token := Token(ctx); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}
