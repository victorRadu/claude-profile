package cli

import (
	"fmt"
	"os"

	"github.com/victorRadu/claude-profile/internal/profile"
	"github.com/victorRadu/claude-profile/internal/statusline"
)

// statuslineCmd implements the statusline subcommand. With no arguments it
// renders the line (invoked by Claude Code, status JSON on stdin); install
// and uninstall manage the statusLine entry in a profile's settings.json.
func (a *App) statuslineCmd(args []string) error {
	if len(args) == 0 {
		color := os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
		return statusline.Render(a.Stdin, a.Stdout, os.Getenv(profile.EnvConfigDir), color)
	}
	verb := args[0]
	if verb != "install" && verb != "uninstall" || len(args) != 2 {
		return fmt.Errorf("usage: claude-profile statusline <install|uninstall> <name>")
	}
	name := args[1]
	if !a.Store.Exists(name) {
		return fmt.Errorf("profile %q does not exist", name)
	}
	dir := a.Store.Dir(name)
	st := a.Style
	if verb == "install" {
		chained, err := statusline.Install(dir, a.Exe)
		if err != nil {
			return err
		}
		if chained {
			a.printf("%s Status line for %s shows profile and model %s\n",
				st.green("✓"), st.bold(st.cyan(name)), st.dim("(your existing status line is kept and shown after it)"))
		} else {
			a.printf("%s Status line for %s shows profile and model\n", st.green("✓"), st.bold(st.cyan(name)))
		}
		return nil
	}
	restored, err := statusline.Uninstall(dir)
	if err != nil {
		return err
	}
	if restored {
		a.printf("%s Removed the claude-profile status line from %s and restored the original one\n", st.green("✓"), st.bold(st.cyan(name)))
	} else {
		a.printf("%s Removed the claude-profile status line from %s\n", st.green("✓"), st.bold(st.cyan(name)))
	}
	return nil
}

// installStatusline is the non-fatal variant used by create.
func (a *App) installStatusline(p profile.Profile) {
	chained, err := statusline.Install(p.Dir, a.Exe)
	if err != nil {
		fmt.Fprintf(a.Stderr, "Warning: could not set up the status line: %v\n", err)
		return
	}
	st := a.Style
	if chained {
		a.printf("%s Status line shows profile and model %s\n", st.green("✓"), st.dim("(copied status line kept, shown after it)"))
	} else {
		a.printf("%s Status line shows profile and model\n", st.green("✓"))
	}
}
