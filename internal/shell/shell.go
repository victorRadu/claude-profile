// Package shell writes per-profile launcher aliases into shell startup files.
//
// All aliases delegate to `claude-profile run <name>`, so the environment
// handling lives in one place and CLAUDE_CONFIG_DIR never leaks into the
// interactive shell session. Every edit is confined to a clearly marked,
// machine-managed block.
package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Shell is one supported shell whose startup file we can manage.
type Shell interface {
	// Name is a human-readable identifier: "bash", "zsh", "powershell".
	Name() string
	// StartupFile is the file that receives the managed block.
	StartupFile() (string, error)
	// AliasKey is the stable prefix identifying a profile's alias line.
	AliasKey(profile string) string
	// AliasLine launches the given profile via the claude-profile binary.
	AliasLine(profile, exe string) string
	// GuardKey and GuardLine redirect plain `claude` to the profile picker.
	GuardKey() string
	GuardLine(exe string) string
	// PathKey and PathLine prepend a directory to PATH (used by the
	// optional claude wrapper).
	PathKey() string
	PathLine(dir string) string
	// SelfAliasLine defines a user-chosen short alias for the
	// claude-profile binary itself. It must contain SelfAliasTag.
	SelfAliasLine(name, exe string) string
}

// SelfAliasTag marks the short-alias line inside the managed block, so it
// can be found and replaced even when the user renames the alias.
const SelfAliasTag = "# claude-profile shortcut"

// Detect returns the shells that look active on this machine.
// CLAUDE_PROFILE_RC forces a single bash-style rc file (useful in tests
// and unusual setups).
func Detect() []Shell {
	if rc := os.Getenv("CLAUDE_PROFILE_RC"); rc != "" {
		return []Shell{Bash{rc: rc}}
	}
	if runtime.GOOS == "windows" {
		return []Shell{PowerShell{}}
	}
	var shells []Shell
	if s := (Zsh{}); s.isActive() {
		shells = append(shells, s)
	}
	if s := (Bash{}); s.isActive() {
		shells = append(shells, s)
	}
	if len(shells) == 0 {
		shells = append(shells, Bash{}) // sensible default on Unix
	}
	if s := (PowerShell{}); s.isActive() {
		shells = append(shells, s)
	}
	return shells
}

// ---------------------------------------------------------------- bash ----

// Bash manages ~/.bashrc (or ~/.bash_profile on macOS, where terminals
// start login shells).
type Bash struct {
	rc string // optional override
}

// Name implements Shell.
func (Bash) Name() string { return "bash" }

// StartupFile implements Shell.
func (b Bash) StartupFile() (string, error) {
	if b.rc != "" {
		return b.rc, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" {
		if p := filepath.Join(home, ".bash_profile"); fileExists(p) {
			return p, nil
		}
	}
	return filepath.Join(home, ".bashrc"), nil
}

// AliasKey implements Shell.
func (Bash) AliasKey(profile string) string { return "alias claude-" + profile + "=" }

// AliasLine implements Shell. The exe path is double-quoted inside the
// alias so paths containing spaces survive word splitting.
func (Bash) AliasLine(profile, exe string) string {
	return fmt.Sprintf(`alias claude-%s='"%s" run %s'`, profile, exe, profile)
}

// GuardKey implements Shell.
func (Bash) GuardKey() string { return "alias claude=" }

// GuardLine implements Shell.
func (Bash) GuardLine(exe string) string {
	return fmt.Sprintf(`alias claude='"%s" run --'`, exe)
}

// PathKey implements Shell.
func (Bash) PathKey() string { return "export PATH=" }

// PathLine implements Shell.
func (Bash) PathLine(dir string) string {
	return fmt.Sprintf(`export PATH="%s:$PATH"`, dir)
}

// SelfAliasLine implements Shell.
func (Bash) SelfAliasLine(name, exe string) string {
	return fmt.Sprintf(`alias %s='"%s"'  %s`, name, exe, SelfAliasTag)
}

func (b Bash) isActive() bool {
	if filepath.Base(os.Getenv("SHELL")) == "bash" {
		return true
	}
	f, err := b.StartupFile()
	return err == nil && fileExists(f)
}

// ----------------------------------------------------------------- zsh ----

// Zsh manages ${ZDOTDIR:-$HOME}/.zshrc.
type Zsh struct{}

// Name implements Shell.
func (Zsh) Name() string { return "zsh" }

// StartupFile implements Shell.
func (Zsh) StartupFile() (string, error) {
	dir := os.Getenv("ZDOTDIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = home
	}
	return filepath.Join(dir, ".zshrc"), nil
}

// AliasKey implements Shell.
func (Zsh) AliasKey(profile string) string { return "alias claude-" + profile + "=" }

// AliasLine implements Shell.
func (Zsh) AliasLine(profile, exe string) string {
	return fmt.Sprintf(`alias claude-%s='"%s" run %s'`, profile, exe, profile)
}

// GuardKey implements Shell.
func (Zsh) GuardKey() string { return "alias claude=" }

// GuardLine implements Shell.
func (Zsh) GuardLine(exe string) string {
	return fmt.Sprintf(`alias claude='"%s" run --'`, exe)
}

// PathKey implements Shell.
func (Zsh) PathKey() string { return "export PATH=" }

// PathLine implements Shell.
func (Zsh) PathLine(dir string) string {
	return fmt.Sprintf(`export PATH="%s:$PATH"`, dir)
}

// SelfAliasLine implements Shell.
func (Zsh) SelfAliasLine(name, exe string) string {
	return fmt.Sprintf(`alias %s='"%s"'  %s`, name, exe, SelfAliasTag)
}

func (z Zsh) isActive() bool {
	if filepath.Base(os.Getenv("SHELL")) == "zsh" {
		return true
	}
	f, err := z.StartupFile()
	return err == nil && fileExists(f)
}

// ---------------------------------------------------------- powershell ----

// PowerShell manages the PowerShell $PROFILE script. Aliases become
// functions because PowerShell aliases cannot carry arguments.
type PowerShell struct{}

// Name implements Shell.
func (PowerShell) Name() string { return "powershell" }

// StartupFile implements Shell.
func (PowerShell) StartupFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		// PowerShell 7+ default; PowerShell 5.1 users can set CLAUDE_PROFILE_RC.
		return filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"), nil
	}
	return filepath.Join(home, ".config", "powershell", "Microsoft.PowerShell_profile.ps1"), nil
}

// AliasKey implements Shell.
func (PowerShell) AliasKey(profile string) string {
	return "function claude-" + profile + " "
}

// AliasLine implements Shell.
func (PowerShell) AliasLine(profile, exe string) string {
	return fmt.Sprintf("function claude-%s { & '%s' run %s @args }", profile, exe, profile)
}

// GuardKey implements Shell.
func (PowerShell) GuardKey() string { return "function claude " }

// GuardLine implements Shell.
func (PowerShell) GuardLine(exe string) string {
	return fmt.Sprintf("function claude { & '%s' run -- @args }", exe)
}

// PathKey implements Shell.
func (PowerShell) PathKey() string { return "$env:Path =" }

// PathLine implements Shell.
func (PowerShell) PathLine(dir string) string {
	return fmt.Sprintf("$env:Path = '%s;' + $env:Path", dir)
}

// SelfAliasLine implements Shell.
func (PowerShell) SelfAliasLine(name, exe string) string {
	return fmt.Sprintf("function %s { & '%s' @args }  %s", name, exe, SelfAliasTag)
}

func (p PowerShell) isActive() bool {
	if runtime.GOOS == "windows" {
		return true
	}
	f, err := p.StartupFile()
	return err == nil && fileExists(f)
}

// ------------------------------------------------------------- helpers ----

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// InstallAlias writes the launcher alias for a profile into sh's startup file.
func InstallAlias(sh Shell, profile, exe string) (string, error) {
	file, err := sh.StartupFile()
	if err != nil {
		return "", err
	}
	return file, SetLine(file, sh.AliasKey(profile), sh.AliasLine(profile, exe))
}

// RemoveAlias deletes the launcher alias for a profile from sh's startup file.
func RemoveAlias(sh Shell, profile string) error {
	file, err := sh.StartupFile()
	if err != nil {
		return err
	}
	return RemoveLine(file, sh.AliasKey(profile))
}

// InstallPath prepends dir to PATH in sh's startup file.
func InstallPath(sh Shell, dir string) (string, error) {
	file, err := sh.StartupFile()
	if err != nil {
		return "", err
	}
	return file, SetLine(file, sh.PathKey(), sh.PathLine(dir))
}

// RemovePath deletes the PATH line from sh's startup file.
func RemovePath(sh Shell) error {
	file, err := sh.StartupFile()
	if err != nil {
		return err
	}
	return RemoveLine(file, sh.PathKey())
}
