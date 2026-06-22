package shellgen

import (
	"context"
	"fmt"

	envdriver "workspaced/pkg/driver/env"
)

// GenerateDaemon generates daemon startup code
func GenerateDaemon() (string, error) {
	return `# Start workspaced daemon if available
if command -v workspaced >/dev/null 2>&1; then
	(workspaced daemon --try &) &>/dev/null
fi
`, nil
}

// GenerateFlags generates shell init flags
func GenerateFlags(ctx context.Context) (string, error) {
	root, _ := envdriver.GetDotfilesRoot(ctx)
	return fmt.Sprintf(`# Flag to indicate workspaced shell init is being used
export WORKSPACED_SHELL_INIT=1
export SD_ROOT=%q/bin
export DOTFILES=%q
export NIXCFG_ROOT_PATH=%q
`, root, root, root), nil
}
