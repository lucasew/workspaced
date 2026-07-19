package sh

import "github.com/lucasew/workspaced/pkg/driver/shell"

func init() {
	shell.RegisterWhich("shell_sh", "POSIX sh", "sh")
}
