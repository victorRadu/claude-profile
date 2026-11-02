//go:build !windows

package cli

import (
	"os"
	"syscall"

	"github.com/victorRadu/claude-profile/internal/profile"
)

// launchClaudeAt replaces the current process with the claude binary at
// path, so signals, terminal control and the exit code are handed over
// cleanly. An empty configDir leaves the environment completely untouched.
func launchClaudeAt(path, configDir string, args []string) error {
	env := os.Environ()
	if configDir != "" {
		env = append(env, profile.EnvConfigDir+"="+configDir)
	}
	return syscall.Exec(path, append([]string{path}, args...), env)
}
