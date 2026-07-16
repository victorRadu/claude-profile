package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/victorRadu/claude-profile/internal/profile"
	"github.com/victorRadu/claude-profile/internal/shell"
)

// The optional wrapper puts a tiny `claude` shim ahead of the real binary
// in PATH. Its contract is strict:
//
//	bound directory                → that profile, announced in a banner
//	unbound + interactive terminal → the profile picker
//	unbound + non-interactive      → the real claude, completely untouched
//
// So scripts, IDEs and CI in unbound directories always get byte-identical
// stock behavior, while humans can never silently land in the wrong profile.

// shimDir lives inside the profile root; its name is not a valid profile
// name, so listings never show it.
func (a *App) shimDir() string { return filepath.Join(a.Store.Root, ".bin") }

func (a *App) shimPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(a.shimDir(), "claude.cmd")
	}
	return filepath.Join(a.shimDir(), "claude")
}

func (a *App) wrap(args []string) error {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "install":
		return a.wrapInstall()
	case "uninstall":
		return a.wrapUninstall()
	case "status":
		return a.wrapStatus()
	default:
		return fmt.Errorf("usage: claude-profile wrap <install|uninstall|status>")
	}
}

func (a *App) wrapInstall() error {
	if err := os.MkdirAll(a.shimDir(), 0o755); err != nil {
		return err
	}
	var content string
	if runtime.GOOS == "windows" {
		content = "@echo off\r\n" +
			"rem claude-profile wrapper - managed by 'claude-profile wrap'\r\n" +
			"\"" + a.Exe + "\" wrap-exec %*\r\n" +
			"exit /b %errorlevel%\r\n"
	} else {
		content = "#!/bin/sh\n" +
			"# claude-profile wrapper — managed by 'claude-profile wrap'\n" +
			"exec \"" + a.Exe + "\" wrap-exec \"$@\"\n"
	}
	if err := os.WriteFile(a.shimPath(), []byte(content), 0o755); err != nil {
		return err
	}
	a.printf("%s Installed wrapper %s\n", a.Style.green("✓"), a.Style.dim("("+displayPath(a.shimPath())+")"))

	for _, sh := range shell.Detect() {
		file, err := shell.InstallPath(sh, a.shimDir())
		if err != nil {
			fmt.Fprintf(a.Stderr, "Warning: could not update %s PATH: %v\n", sh.Name(), err)
			continue
		}
		a.printf("%s Prepended it to PATH for %s %s\n", a.Style.green("✓"), sh.Name(), a.Style.dim("("+displayPath(file)+")"))
	}

	a.printf("\nOpen a new terminal, then verify with: %s\n", a.Style.bold("claude-profile wrap status"))
	a.printf("%s\n", a.Style.dim("Bound folders resolve their profile; unbound interactive shells get the picker;"))
	a.printf("%s\n", a.Style.dim("scripts and IDEs in unbound folders get the real claude, completely untouched."))
	return nil
}

func (a *App) wrapUninstall() error {
	for _, sh := range shell.Detect() {
		if err := shell.RemovePath(sh); err != nil {
			fmt.Fprintf(a.Stderr, "Warning: could not update %s PATH: %v\n", sh.Name(), err)
		}
	}
	if err := os.Remove(a.shimPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = os.Remove(a.shimDir()) // only removed when empty
	a.printf("%s Removed the wrapper and its PATH entries. Open a new terminal to apply.\n", a.Style.green("✓"))
	return nil
}

func (a *App) wrapStatus() error {
	st := a.Style
	shim := a.shimPath()
	if _, err := os.Stat(shim); err != nil {
		a.printf("Wrapper:     %s %s\n", "not installed", st.dim("(install with: claude-profile wrap install)"))
		return nil
	}
	a.printf("Wrapper:     %s %s\n", st.green("installed"), st.dim("("+displayPath(shim)+")"))

	// A shim written by a binary that has since moved would forward into
	// the void; catch that before the user hits a cryptic shell error.
	if content, err := os.ReadFile(shim); err == nil && !strings.Contains(string(content), a.Exe) {
		a.printf("%s the wrapper was installed by a different claude-profile binary —\n", st.yellow("Warning:"))
		a.printf("         re-run 'claude-profile wrap install' to repair it.\n")
	}

	resolved, err := exec.LookPath("claude")
	switch {
	case err != nil:
		a.printf("Active:      %s\n", st.yellow("no 'claude' in PATH at all"))
	case resolved == shim:
		a.printf("Active:      %s %s\n", st.green("yes"), st.dim("('claude' resolves to the wrapper)"))
	default:
		a.printf("Active:      %s %s\n", st.yellow("no"), st.dim("('claude' resolves to "+displayPath(resolved)+" — open a new terminal?)"))
	}

	realPath, err := findRealClaude(a.shimDir())
	if err != nil {
		a.printf("Forwards to: %s\n", st.yellow("real claude not found — is Claude Code installed?"))
		return nil
	}
	a.printf("Forwards to: %s\n", displayPath(realPath))
	return nil
}

// wrapExec is the hidden command the shim invokes in place of claude.
func (a *App) wrapExec(args []string) error {
	realPath, err := findRealClaude(a.shimDir())
	if err != nil {
		return fmt.Errorf("the real claude was not found in PATH — is Claude Code installed?")
	}
	// Route every launch (banner path and picker) through the real binary,
	// never back through PATH, which would hit the shim again.
	a.Launch = func(configDir string, claudeArgs []string) error {
		return a.LaunchAt(realPath, configDir, claudeArgs)
	}

	if dir, err := a.cwd(); err == nil {
		res, ok, rerr := profile.Resolve(dir)
		if rerr != nil {
			return rerr // an invalid marker is loud, never skipped
		}
		if ok {
			if !a.Store.Exists(res.Name) {
				return fmt.Errorf("%s in %s names profile %q, which does not exist; create it with: claude-profile create %s",
					profile.MarkerName, displayPath(res.MarkerDir), res.Name, res.Name)
			}
			return a.launchProfile(res.Name, res, args)
		}
	}

	if a.Interactive {
		if profiles, err := a.Store.List(); err == nil && len(profiles) > 0 {
			return a.pickAndLaunch(args)
		}
	}

	// Transparent pass-through: byte-identical stock behavior.
	return a.LaunchAt(realPath, "", args)
}

// findRealClaude locates claude in PATH while ignoring the wrapper's own
// directory, so the shim can never recurse into itself. Directories are
// compared with symlinks resolved: a PATH entry that reaches the shim dir
// through a link must be skipped too, or the shim would spawn itself in a
// loop.
func findRealClaude(shimDir string) (string, error) {
	shimResolved := resolvePath(shimDir)
	oldPath := os.Getenv("PATH")
	var kept []string
	for _, d := range filepath.SplitList(oldPath) {
		if resolvePath(d) == shimResolved {
			continue
		}
		kept = append(kept, d)
	}
	// Setenv on an existing variable cannot realistically fail; the value
	// is restored either way.
	_ = os.Setenv("PATH", strings.Join(kept, string(os.PathListSeparator)))
	defer func() { _ = os.Setenv("PATH", oldPath) }()

	found, err := exec.LookPath("claude")
	if err != nil {
		return "", err
	}
	// Last line of defense against recursion.
	if resolvePath(filepath.Dir(found)) == shimResolved {
		return "", fmt.Errorf("only the wrapper itself was found in PATH")
	}
	return found, nil
}

// resolvePath normalizes a directory for identity comparison, following
// symlinks where possible.
func resolvePath(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}
