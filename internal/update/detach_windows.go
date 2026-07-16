//go:build windows

package update

import (
	"os/exec"
	"syscall"
)

// Windows console/process creation flags (Win32 CREATE_* constants).
const (
	createNewProcessGroup = 0x00000200
	createNoWindow        = 0x08000000
)

// detachAttrs starts the background refresher without a console window and
// in its own process group, so it survives the foreground process.
func detachAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup | createNoWindow}
}
