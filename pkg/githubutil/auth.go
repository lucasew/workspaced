package githubutil

import (
	"context"
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
			token = envToken
			return
		}

		if !execdriver.IsBinaryAvailable(ctx, "gh") {
			return
		}

		cmd, err := execdriver.Run(ctx, "gh", "auth", "token")
		if err != nil {
			return
		}
		out, err := cmd.Output()
		if err != nil {
			return
		}
		token = strings.TrimSpace(string(out))
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
