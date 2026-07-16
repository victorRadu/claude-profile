//go:build !windows

package update

import "os"

// swap atomically renames the new binary over the current one. On Unix a
// running binary keeps executing its old inode, so this is safe mid-flight.
func swap(exePath, newPath string) error {
	return os.Rename(newPath, exePath)
}

// CleanupOld is a no-op on Unix; there is never a leftover .old binary.
func CleanupOld(string) {}
