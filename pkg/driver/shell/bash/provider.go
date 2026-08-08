package bash

import "workspaced/pkg/driver/shell"

func init() {
	shell.RegisterWhich("shell_bash", "Bash", "bash")
}
