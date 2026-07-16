// Package cli implements the claude-profile command-line interface.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/victorRadu/claude-profile/internal/profile"
)

// App carries the dependencies of every command, so tests can substitute
// streams, the profile store and the launcher.
type App struct {
	Store   *profile.Store
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Version string
	// Exe is the path used in generated aliases to invoke this binary.
	Exe string
	// Launch replaces the process (or spawns claude) with configDir set.
	Launch func(configDir string, args []string) error
	// LaunchAt launches a specific claude binary; an empty configDir means
	// "leave the environment untouched" (used by the optional wrapper).
	LaunchAt func(path, configDir string, args []string) error
	// Interactive reports whether stdin is a terminal.
	Interactive bool
	// WorkDir overrides os.Getwd for marker resolution (used in tests).
	WorkDir string
	// Style renders colors; the zero value produces plain output.
	Style palette

	scanner *bufio.Scanner // lazily initialized; reused across prompts
	// rawPending holds bytes read in raw mode but not yet consumed, so a
	// keystroke burst spanning two prompts (menu then confirm) is not lost.
	rawPending []byte
}

// Run executes the CLI and returns a process exit code.
func Run(args []string, version string) int {
	root, err := profile.DefaultRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}
	exe, err := os.Executable()
	if err != nil {
		exe = "claude-profile"
	}
	app := &App{
		Store:       &profile.Store{Root: root},
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Version:     version,
		Exe:         exe,
		Launch:      launchClaude,
		LaunchAt:    launchClaudeAt,
		Interactive: isTerminal(os.Stdin),
		Style:       newPalette(os.Stdout),
	}
	return app.Run(args)
}

// Run dispatches to a subcommand and returns an exit code.
func (a *App) Run(args []string) int {
	cmd := "help"
	if len(args) > 0 {
		cmd = args[0]
		args = args[1:]
	}
	// `claude-profile <cmd> --help` shows the command's help page.
	if _, known := helpTopics[cmd]; known && len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		if err := a.printHelp(cmd); err != nil {
			fmt.Fprintln(a.Stderr, "Error:", err)
			return 1
		}
		return 0
	}
	var err error
	switch cmd {
	case "create":
		err = a.create(args)
	case "list", "ls":
		err = a.list()
	case "run":
		err = a.run(args)
	case "link":
		err = a.link(args)
	case "unlink":
		err = a.unlink()
	case "status":
		err = a.status()
	case "alias":
		err = a.aliasCmd(args)
	case "wrap":
		err = a.wrap(args)
	case "wrap-exec": // hidden: invoked by the wrapper shim, not by users
		err = a.wrapExec(args)
	case "remove", "rm":
		err = a.remove(args)
	case "version", "-v", "--version":
		fmt.Fprintln(a.Stdout, "claude-profile", a.Version)
	case "help", "-h", "--help":
		if len(args) > 0 {
			err = a.printHelp(args[0])
		} else {
			a.printUsage(a.Stdout)
		}
	default:
		fmt.Fprintf(a.Stderr, "Error: unknown command %q\n\n", cmd)
		a.printUsage(a.Stderr)
		return 2
	}
	if err != nil {
		fmt.Fprintln(a.Stderr, "Error:", err)
		return 1
	}
	return 0
}

func (a *App) printf(format string, args ...any) {
	fmt.Fprintf(a.Stdout, format, args...)
}

// ask prints a prompt and reads one trimmed line from stdin.
func (a *App) ask(prompt string) string {
	fmt.Fprint(a.Stdout, prompt)
	if a.scanner == nil {
		a.scanner = bufio.NewScanner(a.Stdin)
	}
	if !a.scanner.Scan() {
		return ""
	}
	return strings.TrimSpace(a.scanner.Text())
}

// confirm asks a yes/no question: one keypress on a real terminal, a
// typed line elsewhere. No is always the default.
func (a *App) confirm(prompt string) bool {
	return a.confirmKey(prompt)
}

func (a *App) confirmLine(prompt string) bool {
	reply := a.ask(prompt + " [y/N] ")
	return reply == "y" || reply == "Y" || reply == "yes"
}
