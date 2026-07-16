//go:build !windows

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/victorRadu/claude-profile/internal/profile"
)

// launchClaude finds claude in PATH and hands the process over to it.
func launchClaude(configDir string, args []string) error {
	path, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found in PATH — install Claude Code first (https://docs.claude.com/en/docs/claude-code)")
	}
	return launchClaudeAt(path, configDir, args)
}

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
