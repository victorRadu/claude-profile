package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/victorRadu/claude-profile/internal/profile"
)

// cwd returns the working directory, honoring the test override.
func (a *App) cwd() (string, error) {
	if a.WorkDir != "" {
		return a.WorkDir, nil
	}
	return os.Getwd()
}

// launchProfile prints the loud one-line banner and launches Claude Code.
// The banner goes to stderr so it never corrupts piped claude output.
func (a *App) launchProfile(name string, res profile.Resolution, args []string) error {
	dir := a.Store.Dir(name)
	st := a.Style
	fmt.Fprintf(a.Stderr, "%s profile: %s %s %s\n",
		st.cyan("◆"), st.bold(st.cyan(name)), st.dim("("+displayPath(dir)+")"), st.dim("— "+via(res)))
	return a.Launch(dir, args)
}

// via renders the resolution explanation with home-shortened paths.
func via(res profile.Resolution) string {
	if res.Source == "marker" {
		return "via " + profile.MarkerName + " in " + displayPath(res.MarkerDir)
	}
	return res.Via()
}

// link binds the current directory to a profile via a marker file.
func (a *App) link(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: claude-profile link <name>")
	}
	name := args[0]
	if err := profile.ValidateName(name); err != nil {
		return err
	}
	if !a.Store.Exists(name) {
		return fmt.Errorf("profile %q does not exist; create it with: claude-profile create %s", name, name)
	}
	dir, err := a.cwd()
	if err != nil {
		return err
	}
	marker, err := profile.WriteMarker(dir, name)
	if err != nil {
		return err
	}
	a.printf("%s Linked %s to profile %s %s\n", a.Style.green("✓"), displayPath(dir), a.Style.bold(a.Style.cyan(name)), a.Style.dim("("+displayPath(marker)+")"))
	a.printf("'claude-profile run' (and a guarded plain 'claude') here now uses %q.\n", name)
	return nil
}

// unlink removes the marker from the current directory.
func (a *App) unlink() error {
	dir, err := a.cwd()
	if err != nil {
		return err
	}
	removed, err := profile.RemoveMarker(dir)
	if err != nil {
		return err
	}
	if removed {
		a.printf("Removed %s from %s\n", profile.MarkerName, displayPath(dir))
		return nil
	}
	// Help the user find a marker further up instead of failing silently.
	if res, ok, _ := profile.Resolve(dir); ok {
		return fmt.Errorf("no %s in %s, but one exists in %s — remove it there",
			profile.MarkerName, displayPath(dir), displayPath(res.MarkerDir))
	}
	a.printf("No %s found in %s — nothing to do.\n", profile.MarkerName, displayPath(dir))
	return nil
}

// status answers "which profile would I get here, and why?".
func (a *App) status() error {
	dir, err := a.cwd()
	if err != nil {
		return err
	}
	st := a.Style
	a.printf("%s  %s\n", st.bold("Profile root: "), displayPath(a.Store.Root))
	res, ok, err := profile.Resolve(dir)
	if err != nil {
		return err
	}
	if !ok {
		a.printf("%s no %s binding — 'run' here opens the picker\n", st.bold("This directory:"), profile.MarkerName)
		a.printf("%s\n", st.dim("Bind one with: claude-profile link <name>"))
		return nil
	}
	a.printf("%s %q %s\n", st.bold("This directory:"), res.Name, st.dim(via(res)))
	if !a.Store.Exists(res.Name) {
		a.printf("%s profile %q does not exist — create it with: claude-profile create %s\n",
			st.yellow("Warning:"), res.Name, res.Name)
		return nil
	}
	p := profile.Profile{Name: res.Name, Dir: a.Store.Dir(res.Name)}
	if loggedIn, known := p.LoggedIn(); known {
		if loggedIn {
			a.printf("%s          %s\n", st.bold("Login:"), st.green("logged in"))
		} else {
			a.printf("%s          %s %s\n", st.bold("Login:"), st.yellow("not logged in"), st.dim("(run the profile and use /login)"))
		}
	}
	return nil
}

// runResolved handles `run` without a profile name: directory marker
// first, otherwise the interactive picker. Never a silent default.
func (a *App) runResolved(args []string) error {
	dir, err := a.cwd()
	if err != nil {
		return err
	}
	res, ok, err := profile.Resolve(dir)
	if err != nil {
		return err
	}
	if ok {
		if !a.Store.Exists(res.Name) {
			return fmt.Errorf("%s in %s names profile %q, which does not exist; create it with: claude-profile create %s",
				profile.MarkerName, displayPath(res.MarkerDir), res.Name, res.Name)
		}
		return a.launchProfile(res.Name, res, args)
	}
	if !a.Interactive {
		return fmt.Errorf("no %s binding here and no terminal for the picker — use 'claude-profile run <name>' or 'claude-profile link <name>'", profile.MarkerName)
	}
	return a.pickAndLaunch(args)
}

// pickAndLaunch shows the interactive picker and launches the choice.
func (a *App) pickAndLaunch(args []string) error {
	profiles, err := a.Store.List()
	if err != nil {
		return err
	}
	if len(profiles) == 0 {
		a.printf("No profiles yet. Create one with: claude-profile create <name>\n")
		return nil
	}
	names := make([]string, len(profiles))
	for i, p := range profiles {
		names[i] = p.Name
	}
	idx, err := a.selectFrom("Start which profile?", names)
	if err != nil || idx < 0 {
		return err
	}
	name := profiles[idx].Name
	return a.launchProfile(name, profile.Resolution{Name: name, Source: "picker"}, args)
}

// displayPath shortens the home directory to ~ for output.
func displayPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}
