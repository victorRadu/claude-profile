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
//
// `run` is the implicit default action: with no arguments, or with a first
// argument that is not a known subcommand, the whole invocation is handed to
// run. That makes `claude-profile <profile> [args]` (and the `claudep` short
// alias) launch straight into a profile, while `create`, `list`, etc. still
// dispatch as named subcommands.
func (a *App) Run(args []string) int {
	// No subcommand: resolve the directory binding, else the picker —
	// never a silent default, and always announced by run's banner.
	if len(args) == 0 {
		return a.exit(a.run(nil))
	}
	cmd := args[0]
	rest := args[1:]
	// `claude-profile <cmd> --help` shows the command's help page.
	if _, known := helpTopics[cmd]; known && len(rest) > 0 && (rest[0] == "-h" || rest[0] == "--help") {
		return a.exit(a.printHelp(cmd))
	}
	var err error
	switch cmd {
	case "create":
		err = a.create(rest)
	case "list", "ls":
		err = a.list()
	case "run":
		err = a.run(rest)
	case "link":
		err = a.link(rest)
	case "unlink":
		err = a.unlink()
	case "status":
		err = a.status()
	case "alias":
		err = a.aliasCmd(rest)
	case "wrap":
		err = a.wrap(rest)
	case "wrap-exec": // hidden: invoked by the wrapper shim, not by users
		err = a.wrapExec(rest)
	case "remove", "rm":
		err = a.remove(rest)
	case "version", "-v", "--version":
		fmt.Fprintln(a.Stdout, "claude-profile", a.Version)
	case "help", "-h", "--help":
		if len(rest) > 0 {
			err = a.printHelp(rest[0])
		} else {
			a.printUsage(a.Stdout)
		}
	default:
		// Not a known subcommand: treat the entire invocation as run args.
		// splitRunArgs still refuses to guess a bad profile name.
		err = a.run(args)
	}
	return a.exit(err)
}

// exit prints err (if any) to stderr and returns the process exit code.
func (a *App) exit(err error) int {
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
