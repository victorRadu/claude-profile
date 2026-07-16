//go:build !windows

package cli

import "os"

// enableVT is a no-op on Unix terminals, which speak ANSI natively.
func enableVT(*os.File) bool { return true }
