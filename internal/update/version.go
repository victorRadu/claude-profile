// Package update implements the version check, its on-disk cache, and the
// self-update mechanism. See docs/updates.md for the full strategy.
package update

import (
	"strconv"
	"strings"
)

// IsDevBuild reports whether v is a development build ("dev", or any
// git-describe/-local suffix that keeps it from parsing as a release),
// which never checks for or installs updates.
func IsDevBuild(v string) bool {
	v = Canonical(v)
	if v == "" || v == "dev" {
		return true
	}
	_, ok := parseVersion(v)
	return !ok
}

// Newer reports whether latest is a strictly newer release than current.
// Anything unparsable compares as "not newer" — we never nag on guesses.
func Newer(latest, current string) bool {
	l, ok := parseVersion(latest)
	if !ok {
		return false
	}
	c, ok := parseVersion(current)
	if !ok {
		return false
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// Canonical strips a leading "v" for display and asset names.
func Canonical(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// parseVersion parses "1.2.3" or "v1.2.3" into a comparable triple.
// Pre-release or build suffixes make a version unparsable on purpose.
func parseVersion(v string) ([3]int, bool) {
	v = Canonical(v)
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}
