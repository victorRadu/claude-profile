//go:build windows

package update

import "os"

// swap replaces the current binary on Windows, where a running executable
// cannot be overwritten but can be renamed: move the running binary aside
// to .old, move the new one into place. The leftover .old is removed
// best-effort on the next start (CleanupOld).
func swap(exePath, newPath string) error {
	old := exePath + ".old"
	_ = os.Remove(old) // a leftover from a previous update, if unlocked
	if err := os.Rename(exePath, old); err != nil {
		return err
	}
	if err := os.Rename(newPath, exePath); err != nil {
		// Put the original back so the install is never left broken.
		_ = os.Rename(old, exePath)
		return err
	}
	return nil
}

// CleanupOld removes the .old binary a previous self-update left behind.
// It fails silently while that binary is still locked by a running process.
func CleanupOld(exePath string) {
	_ = os.Remove(exePath + ".old")
}
