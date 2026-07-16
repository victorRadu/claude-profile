//go:build darwin

package cli

import (
	"os"
	"syscall"
	"unsafe"
)

// isTerminal reports whether f is a real terminal (isatty). See
// terminal_linux.go for why a character-device check is insufficient.
func isTerminal(f *os.File) bool {
	var termios syscall.Termios
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, f.Fd(),
		syscall.TIOCGETA, uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	return errno == 0
}

// enableRaw switches the terminal to raw mode for the interactive select
// component. See terminal_linux.go for the rationale.
func enableRaw(f *os.File) (func(), error) {
	fd := f.Fd()
	var old syscall.Termios
	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd,
		syscall.TIOCGETA, uintptr(unsafe.Pointer(&old)), 0, 0, 0); errno != 0 {
		return nil, errno
	}
	raw := old
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd,
		syscall.TIOCSETA, uintptr(unsafe.Pointer(&raw)), 0, 0, 0); errno != 0 {
		return nil, errno
	}
	return func() {
		syscall.Syscall6(syscall.SYS_IOCTL, fd,
			syscall.TIOCSETA, uintptr(unsafe.Pointer(&old)), 0, 0, 0)
	}, nil
}
