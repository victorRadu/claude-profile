//go:build !linux && !darwin && !windows

package cli

import "os"

// isTerminal falls back to a character-device check on platforms without
// a dedicated isatty implementation.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// enableRaw is unsupported here; callers fall back to numbered selection.
func enableRaw(*os.File) (func(), error) {
	return nil, errNoRawMode
}
