//go:build windows

package cli

import (
	"errors"
	"os"
	"os/exec"

	"github.com/victorRadu/claude-profile/internal/profile"
)

// launchClaudeAt runs the claude binary at path as a child process.
// Windows has no exec(2), so we spawn and mirror the exit code. An empty
// configDir leaves the environment completely untouched.
func launchClaudeAt(path, configDir string, args []string) error {
	cmd := exec.Command(path, args...)
	cmd.Env = os.Environ()
	if configDir != "" {
		cmd.Env = append(cmd.Env, profile.EnvConfigDir+"="+configDir)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	return err
}
