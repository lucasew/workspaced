package bash

import "github.com/lucasew/workspaced/pkg/driver/shell"

func init() {
	shell.RegisterWhich("shell_bash", "Bash", "bash")
}
