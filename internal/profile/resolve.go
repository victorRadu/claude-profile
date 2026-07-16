package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MarkerName is the per-directory binding file. It contains a single
// profile name and is looked up from the working directory upward,
// like .git or .nvmrc.
const MarkerName = ".claude-profile"

// Resolution explains how a profile name was determined.
type Resolution struct {
	Name      string
	Source    string // "explicit", "marker" or "picker"
	MarkerDir string // directory containing the marker, when Source == "marker"
}

// Via renders a human-readable explanation for the launch banner.
func (r Resolution) Via() string {
	switch r.Source {
	case "marker":
		return "via " + MarkerName + " in " + r.MarkerDir
	case "picker":
		return "via interactive picker"
	default:
		return "via explicit argument"
	}
}

// Resolve walks from startDir toward the filesystem root looking for a
// marker file. It returns ok=false when no marker exists. A marker with
// an invalid profile name is an error, never silently ignored.
func Resolve(startDir string) (Resolution, bool, error) {
	dir := startDir
	for {
		marker := filepath.Join(dir, MarkerName)
		if info, err := os.Stat(marker); err == nil && !info.IsDir() {
			b, err := os.ReadFile(marker)
			if err != nil {
				return Resolution{}, false, fmt.Errorf("reading %s: %w", marker, err)
			}
			name := strings.TrimSpace(strings.SplitN(string(b), "\n", 2)[0])
			if err := ValidateName(name); err != nil {
				return Resolution{}, false, fmt.Errorf("%s: %w", marker, err)
			}
			return Resolution{Name: name, Source: "marker", MarkerDir: dir}, true, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Resolution{}, false, nil
		}
		dir = parent
	}
}

// WriteMarker binds dir to a profile by writing the marker file.
func WriteMarker(dir, name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	marker := filepath.Join(dir, MarkerName)
	return marker, os.WriteFile(marker, []byte(name+"\n"), 0o644)
}

// RemoveMarker deletes the marker in exactly dir (no upward walk).
// It reports whether a marker was removed.
func RemoveMarker(dir string) (bool, error) {
	marker := filepath.Join(dir, MarkerName)
	err := os.Remove(marker)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
