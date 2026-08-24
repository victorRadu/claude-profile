// Package profile implements the profile store: a directory of isolated
// Claude Code configuration directories, one per profile.
package profile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
)

// EnvRoot overrides the default profile root when set.
const EnvRoot = "CLAUDE_PROFILES_DIR"

// EnvConfigDir is the environment variable Claude Code reads to locate its
// configuration directory.
const EnvConfigDir = "CLAUDE_CONFIG_DIR"

var nameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// ErrInvalidName is returned for profile names that could be unsafe in paths
// or shell aliases.
var ErrInvalidName = errors.New("profile name must start with a letter or digit and contain only letters, digits, '-' and '_'")

// Profile describes one managed profile.
type Profile struct {
	Name string
	Dir  string
}

// Store manages profiles under a root directory.
type Store struct {
	Root string
}

// DefaultRoot returns $CLAUDE_PROFILES_DIR or ~/.claude-profiles.
func DefaultRoot() (string, error) {
	if v := os.Getenv(EnvRoot); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".claude-profiles"), nil
}

// DefaultConfigDir returns the standard (non-profile) Claude Code config
// directory, ~/.claude, if it exists.
func DefaultConfigDir() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	dir := filepath.Join(home, ".claude")
	info, err := os.Stat(dir)
	return dir, err == nil && info.IsDir()
}

// maxNameLen keeps profile names comfortably inside filesystem and shell
// limits while staying readable in listings and banners.
const maxNameLen = 64

// ValidateName reports whether name is a legal profile name.
func ValidateName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: %w", name, ErrInvalidName)
	}
	if len(name) > maxNameLen {
		return fmt.Errorf("invalid profile name %q: longer than %d characters", name, maxNameLen)
	}
	return nil
}

// Dir returns the configuration directory for a profile name.
func (s *Store) Dir(name string) string {
	return filepath.Join(s.Root, name)
}

// Exists reports whether the named profile exists.
func (s *Store) Exists(name string) bool {
	info, err := os.Stat(s.Dir(name))
	return err == nil && info.IsDir()
}

// Create makes a new, empty profile and returns it.
func (s *Store) Create(name string) (Profile, error) {
	if err := ValidateName(name); err != nil {
		return Profile{}, err
	}
	if s.Exists(name) {
		return Profile{}, fmt.Errorf("profile %q already exists (%s)", name, s.Dir(name))
	}
	dir := s.Dir(name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Profile{}, fmt.Errorf("creating profile directory: %w", err)
	}
	return Profile{Name: name, Dir: dir}, nil
}

// List returns all profiles, sorted by name.
func (s *Store) List() ([]Profile, error) {
	entries, err := os.ReadDir(s.Root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading profile root: %w", err)
	}
	var out []Profile
	for _, e := range entries {
		// Skip files and directories that cannot be profiles (e.g. the
		// hidden .bin directory holding the optional claude wrapper).
		if !e.IsDir() || ValidateName(e.Name()) != nil {
			continue
		}
		out = append(out, Profile{Name: e.Name(), Dir: s.Dir(e.Name())})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Remove deletes a profile directory and everything in it.
func (s *Store) Remove(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if !s.Exists(name) {
		return fmt.Errorf("profile %q does not exist", name)
	}
	return os.RemoveAll(s.Dir(name))
}

// LoggedIn reports whether a profile appears to be authenticated.
// The second return value is false when this cannot be determined
// (macOS and Windows store credentials outside the config directory).
func (p Profile) LoggedIn() (loggedIn, known bool) {
	if runtime.GOOS != "linux" {
		return false, false
	}
	_, err := os.Stat(filepath.Join(p.Dir, ".credentials.json"))
	return err == nil, true
}

func copyRecursive(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		// Skip symlinks: they may point at credentials or files outside
		// the profile, and copying them would break isolation.
		return nil
	case info.IsDir():
		if err := os.MkdirAll(dst, 0o700); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyRecursive(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	default:
		return copyFile(src, dst, info.Mode().Perm())
	}
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }() // read-only handle; close error is moot
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
