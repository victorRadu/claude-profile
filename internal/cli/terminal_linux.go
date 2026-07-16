//go:build linux

package cli

import (
	"os"
	"syscall"
	"unsafe"
)

// isTerminal reports whether f is a real terminal (isatty). A plain
// character-device check is not enough: /dev/null is a character device
// but must count as non-interactive, or the wrapper would swallow
// scripted invocations.
func isTerminal(f *os.File) bool {
	var termios syscall.Termios
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, f.Fd(),
		syscall.TCGETS, uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	return errno == 0
}

// enableRaw switches the terminal to raw mode (no echo, no line
// buffering, no signal keys) for the interactive select component, and
// returns a restore function. ISIG is cleared so Ctrl-C is delivered as a
// byte we can handle — otherwise it would kill the process and leave the
// terminal in raw mode.
func enableRaw(f *os.File) (func(), error) {
	fd := f.Fd()
	var old syscall.Termios
	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd,
		syscall.TCGETS, uintptr(unsafe.Pointer(&old)), 0, 0, 0); errno != 0 {
		return nil, errno
	}
	raw := old
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd,
		syscall.TCSETS, uintptr(unsafe.Pointer(&raw)), 0, 0, 0); errno != 0 {
		return nil, errno
	}
	return func() {
		syscall.Syscall6(syscall.SYS_IOCTL, fd,
			syscall.TCSETS, uintptr(unsafe.Pointer(&old)), 0, 0, 0)
	}, nil
}
