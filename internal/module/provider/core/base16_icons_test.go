package core

import (
	"testing"

	"github.com/lucasew/workspaced/internal/module"
)

func TestBase16IconsLinuxRegistered(t *testing.T) {
	t.Parallel()
	if _, ok := module.GetCoreModule("base16-icons-linux"); !ok {
		t.Fatal(`GetCoreModule("base16-icons-linux") missing; source file must not be named *_linux.go (implicit GOOS build tag)`)
	}
}
