//go:build windows

package cli

import (
	"os"
	"syscall"
)

// isTerminal reports whether f is attached to a Windows console.
func isTerminal(f *os.File) bool {
	var mode uint32
	return syscall.GetConsoleMode(syscall.Handle(f.Fd()), &mode) == nil
}

// enableRaw puts the console into raw mode with virtual terminal input
// (Windows 10+), so arrow keys arrive as the same ANSI sequences the Unix
// code path parses. Returns an error on legacy consoles, which then get
// the numbered fallback.
func enableRaw(f *os.File) (func(), error) {
	const (
		enableProcessedInput       = 0x0001
		enableLineInput            = 0x0002
		enableEchoInput            = 0x0004
		enableVirtualTerminalInput = 0x0200
	)
	handle := syscall.Handle(f.Fd())
	var old uint32
	if err := syscall.GetConsoleMode(handle, &old); err != nil {
		return nil, err
	}
	raw := old &^ (enableProcessedInput | enableLineInput | enableEchoInput)
	raw |= enableVirtualTerminalInput
	setMode := syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleMode")
	if r, _, err := setMode.Call(uintptr(handle), uintptr(raw)); r == 0 {
		return nil, err
	}
	return func() { _, _, _ = setMode.Call(uintptr(handle), uintptr(old)) }, nil
}
