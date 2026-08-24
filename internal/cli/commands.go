package cli

import (
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/victorRadu/claude-profile/internal/migrate"
	"github.com/victorRadu/claude-profile/internal/profile"
	"github.com/victorRadu/claude-profile/internal/shell"
)

// splitName pulls a leading positional argument off args, so flags may
// appear before or after the name (Go's flag package alone stops parsing
// at the first positional argument).
func splitName(args []string) (name string, rest []string) {
	for i, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a, append(append([]string{}, args[:i]...), args[i+1:]...)
		}
	}
	return "", args
}

func (a *App) create(args []string) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	from := fs.String("from", "", "copy configuration from another profile, or 'default' for ~/.claude")
	noAlias := fs.Bool("no-alias", false, "do not install shell aliases")
	name, rest := splitName(args)
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if name == "" || fs.NArg() != 0 {
		return fmt.Errorf("usage: claude-profile create <name> [--from <profile|default>] [--no-alias]")
	}

	p, err := a.Store.Create(name)
	if err != nil {
		return err
	}
	a.printf("%s Created profile %s %s\n", a.Style.green("✓"), a.Style.bold(a.Style.cyan(p.Name)), a.Style.dim("("+displayPath(p.Dir)+")"))

	if *from == "" && a.Interactive {
		*from = a.offerCopySource(name)
	}
	if *from != "" {
		if err := a.copyInto(p, *from); err != nil {
			return err
		}
	}

	if !*noAlias {
		a.installAliases(name)
		a.offerGuardAlias()
	}

	// After any copy, so a copied statusLine is chained, never overwritten.
	a.installStatusline(p)

	// A new profile has every current feature built in, so no migration
	// may ever run on it (see internal/migrate).
	if err := migrate.StampAll(p, a.Version); err != nil {
		fmt.Fprintf(a.Stderr, "Warning: could not record the profile's feature level: %v\n", err)
	}

	a.printf("\nDone. Open a new terminal (or reload your shell), then run: %s\n", a.Style.bold("claude-"+name))
	a.printf("%s\n", a.Style.dim("On first run, use /login inside Claude Code to authenticate this profile."))
	return nil
}

func (a *App) installAliases(name string) {
	for _, sh := range shell.Detect() {
		file, err := shell.InstallAlias(sh, name, a.Exe)
		if err != nil {
			fmt.Fprintf(a.Stderr, "Warning: could not add %s alias: %v\n", sh.Name(), err)
			continue
		}
		a.printf("%s Added %s alias %s %s\n", a.Style.green("✓"), sh.Name(), a.Style.cyan("claude-"+name), a.Style.dim("("+displayPath(file)+")"))
	}
}

// offerGuardAlias optionally redirects plain `claude` to the profile picker,
// so the default profile is not started by accident. Never overrides an
// existing user alias.
func (a *App) offerGuardAlias() {
	if !a.Interactive {
		return
	}
	shells := shell.Detect()
	needed := false
	for _, sh := range shells {
		file, err := sh.StartupFile()
		if err != nil {
			continue
		}
		if has, _ := shell.HasLine(file, sh.GuardKey()); !has {
			needed = true
		}
	}
	if !needed {
		return
	}
	if !a.confirm("Make plain 'claude' profile-aware (use this directory's binding, else the picker), so you never start the default profile by mistake?") {
		return
	}
	for _, sh := range shells {
		file, err := sh.StartupFile()
		if err != nil {
			continue
		}
		if has, _ := shell.HasLine(file, sh.GuardKey()); has {
			continue
		}
		if err := shell.SetLine(file, sh.GuardKey(), sh.GuardLine(a.Exe)); err != nil {
			fmt.Fprintf(a.Stderr, "Warning: could not update %s: %v\n", file, err)
		}
	}
	a.printf("Plain 'claude' now resolves the profile explicitly (use 'command claude' for the default profile).\n")
}

func (a *App) list() error {
	profiles, err := a.Store.List()
	if err != nil {
		return err
	}
	if len(profiles) == 0 {
		a.printf("No profiles yet. Create one with: claude-profile create <name>\n")
		return nil
	}
	st := a.Style
	for _, p := range profiles {
		dot, status := st.dim("·"), ""
		if loggedIn, known := p.LoggedIn(); known {
			if loggedIn {
				dot, status = st.green("●"), st.dim("logged in")
			} else {
				dot, status = st.yellow("○"), st.yellow("not logged in")
			}
		}
		a.printf("%s %s %s %s\n", dot, pad(st.bold(p.Name), st, 18), pad(st.cyan("claude-"+p.Name), st, 25), status)
	}
	return nil
}

// pad right-pads a styled string to visible width n (ANSI codes are
// invisible, so fmt's %-Ns would misalign colored columns).
func pad(styled string, st palette, n int) string {
	visible := len(styled)
	if st.on {
		visible = len(stripANSI(styled))
	}
	for visible < n {
		styled += " "
		visible++
	}
	return styled
}

func stripANSI(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		out = append(out, s[i])
	}
	return string(out)
}

// run launches Claude Code. With a profile name it is fully explicit;
// without one it resolves the directory binding, then falls back to the
// interactive picker — never to a silent default.
func (a *App) run(args []string) error {
	name, claudeArgs, err := splitRunArgs(args)
	if err != nil {
		return err
	}
	if name == "" {
		return a.runResolved(claudeArgs)
	}
	if !a.Store.Exists(name) {
		return fmt.Errorf("profile %q does not exist — create it with 'claude-profile create %s', or use 'claude-profile run -- %s' to pass %q to claude instead",
			name, name, name, name)
	}
	return a.launchProfile(name, profile.Resolution{Name: name, Source: "explicit"}, claudeArgs)
}

// splitRunArgs separates an optional leading profile name from claude
// arguments. A leading "--" or "-flag" means no name was given. A first
// argument that cannot be a profile name is an error, never a guess.
func splitRunArgs(args []string) (name string, claudeArgs []string, err error) {
	if len(args) == 0 {
		return "", nil, nil
	}
	switch {
	case args[0] == "--":
		return "", args[1:], nil
	case strings.HasPrefix(args[0], "-"):
		return "", args, nil
	}
	if err := profile.ValidateName(args[0]); err != nil {
		return "", nil, fmt.Errorf("%q is not a valid profile name — to pass it to claude, use: claude-profile run [name] -- %s", args[0], args[0])
	}
	rest := args[1:]
	if len(rest) > 0 && rest[0] == "--" {
		rest = rest[1:]
	}
	return args[0], rest, nil
}

// parseChoice validates a 1-based menu selection.
func parseChoice(choice string, limit int) (int, error) {
	n, err := strconv.Atoi(choice)
	if err != nil || n < 1 || n > limit {
		return 0, fmt.Errorf("invalid choice %q", choice)
	}
	return n, nil
}

func (a *App) remove(args []string) error {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	force := fs.Bool("force", false, "delete without confirmation")
	name, rest := splitName(args)
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if name == "" || fs.NArg() != 0 {
		return fmt.Errorf("usage: claude-profile remove <name> [--force]")
	}
	if !a.Store.Exists(name) {
		return fmt.Errorf("profile %q does not exist", name)
	}
	if !*force {
		if !a.Interactive {
			return fmt.Errorf("refusing to delete without confirmation; use --force")
		}
		if !a.confirm(fmt.Sprintf("Delete profile %q and all its data (%s)?", name, a.Store.Dir(name))) {
			a.printf("Aborted.\n")
			return nil
		}
	}
	if err := a.Store.Remove(name); err != nil {
		return err
	}
	for _, sh := range shell.Detect() {
		if err := shell.RemoveAlias(sh, name); err != nil {
			fmt.Fprintf(a.Stderr, "Warning: could not remove %s alias: %v\n", sh.Name(), err)
		}
	}
	a.printf("Removed profile %q and its aliases. Open a new terminal to apply.\n", name)
	return nil
}
