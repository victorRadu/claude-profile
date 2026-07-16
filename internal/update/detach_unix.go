//go:build !windows

package update

import (
	"os/exec"
	"syscall"
)

// detachAttrs makes the background refresher its own session leader, so it
// survives the foreground process (which may exec claude immediately after).
func detachAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
