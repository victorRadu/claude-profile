package cli

import (
	"flag"
	"fmt"

	"github.com/victorRadu/claude-profile/internal/profile"
	"github.com/victorRadu/claude-profile/internal/shell"
)

// aliasCmd manages the user-chosen short alias for the claude-profile
// binary itself (installed as `claudep` by the install scripts). The alias
// lives in the managed shell block, so renaming replaces the old one and
// uninstalling the block removes it.
func (a *App) aliasCmd(args []string) error {
	fs := flag.NewFlagSet("alias", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	remove := fs.Bool("remove", false, "remove the short alias")
	name, rest := splitName(args)
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: claude-profile alias [name] [--remove]")
	}

	switch {
	case *remove:
		return a.aliasRemove()
	case name == "":
		return a.aliasShow()
	default:
		return a.aliasSet(name)
	}
}

func (a *App) aliasSet(name string) error {
	if err := profile.ValidateName(name); err != nil {
		return err
	}
	if name == "claude" {
		return fmt.Errorf("'claude' would shadow Claude Code itself — that job belongs to the guard alias (offered during 'create') or 'claude-profile wrap'")
	}
	for _, sh := range shell.Detect() {
		file, err := sh.StartupFile()
		if err != nil {
			fmt.Fprintf(a.Stderr, "Warning: could not resolve %s startup file: %v\n", sh.Name(), err)
			continue
		}
		if err := shell.SetTaggedLine(file, shell.SelfAliasTag, sh.SelfAliasLine(name, a.Exe)); err != nil {
			fmt.Fprintf(a.Stderr, "Warning: could not update %s: %v\n", file, err)
			continue
		}
		a.printf("%s %s alias %s %s\n", a.Style.green("✓"), sh.Name(), a.Style.bold(a.Style.cyan(name)), a.Style.dim("("+displayPath(file)+")"))
	}
	a.printf("Open a new terminal (or reload your shell), then try: %s\n", a.Style.bold(name+" status"))
	return nil
}

func (a *App) aliasRemove() error {
	for _, sh := range shell.Detect() {
		file, err := sh.StartupFile()
		if err != nil {
			continue
		}
		if err := shell.RemoveTaggedLine(file, shell.SelfAliasTag); err != nil {
			fmt.Fprintf(a.Stderr, "Warning: could not update %s: %v\n", file, err)
		}
	}
	a.printf("%s Removed the short alias. Open a new terminal to apply.\n", a.Style.green("✓"))
	return nil
}

func (a *App) aliasShow() error {
	found := false
	for _, sh := range shell.Detect() {
		file, err := sh.StartupFile()
		if err != nil {
			continue
		}
		line, ok, err := shell.FindTaggedLine(file, shell.SelfAliasTag)
		if err != nil || !ok {
			continue
		}
		found = true
		a.printf("%-11s %s %s\n", sh.Name()+":", line, a.Style.dim("("+displayPath(file)+")"))
	}
	if !found {
		a.printf("No short alias set. Add one with: claude-profile alias claudep\n")
	}
	return nil
}
