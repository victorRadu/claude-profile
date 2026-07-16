//go:build windows

package cli

import (
	"os"
	"syscall"
)

// enableVT turns on ANSI escape sequence processing for the Windows
// console (Windows 10+). Returns false on legacy consoles, which then
// simply get uncolored output.
func enableVT(f *os.File) bool {
	handle := syscall.Handle(f.Fd())
	var mode uint32
	if err := syscall.GetConsoleMode(handle, &mode); err != nil {
		return false
	}
	const enableVirtualTerminalProcessing = 0x0004
	if mode&enableVirtualTerminalProcessing != 0 {
		return true
	}
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleMode")
	r, _, _ := proc.Call(uintptr(handle), uintptr(mode|enableVirtualTerminalProcessing))
	return r != 0
}
