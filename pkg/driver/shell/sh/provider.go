package sh

import "workspaced/pkg/driver/shell"

func init() {
	shell.RegisterWhich("shell_sh", "POSIX sh", "sh")
}
