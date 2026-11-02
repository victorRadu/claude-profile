package cli

import "fmt"

// launchClaude is the default App.Launch: it finds the real claude binary
// and hands over with configDir set. The lookup skips the wrapper's own
// directory, so a launch that already resolved its profile (run <name>, a
// claude-<name> alias, the picker) can never exec the shim and have it
// re-resolve the profile from the current folder's binding — the shim
// would otherwise override the profile the user explicitly asked for.
func (a *App) launchClaude(configDir string, args []string) error {
	path, err := findRealClaude(a.shimDir())
	if err != nil {
		return fmt.Errorf("claude not found in PATH — install Claude Code first (https://docs.claude.com/en/docs/claude-code)")
	}
	return a.LaunchAt(path, configDir, args)
}
