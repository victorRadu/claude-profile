package cli

import (
	"fmt"
	"io"
)

type cmdHelp struct {
	summary  string
	usage    string
	details  string
	options  [][2]string
	examples []string
}

// commandOrder controls the listing in the overview.
var commandOrder = []string{
	"create", "list", "run", "link", "unlink",
	"status", "alias", "wrap", "remove", "version", "help",
}

var helpTopics = map[string]cmdHelp{
	"create": {
		summary: "Create a new profile",
		usage:   "claude-profile create <name> [--from <profile|default>] [--no-alias]",
		details: "Creates an isolated Claude Code configuration directory and installs a\n" +
			"claude-<name> alias into your shell startup files (bash, zsh and\n" +
			"PowerShell are detected automatically). On first launch, use /login\n" +
			"inside Claude Code to authenticate the profile.",
		options: [][2]string{
			{"--from <profile|default>", "Copy settings.json, CLAUDE.md, skills, agents and commands from another profile, or from ~/.claude ('default'). Credentials and history are never copied."},
			{"--no-alias", "Skip installing shell aliases."},
		},
		examples: []string{
			"claude-profile create client-acme --from default",
			"claude-profile create personal",
			"claude-profile create go --from php",
		},
	},
	"list": {
		summary: "List profiles and their login state",
		usage:   "claude-profile list",
		details: "Shows every profile, its launch alias and, where detectable, whether\n" +
			"it is logged in.",
	},
	"run": {
		summary: "Launch Claude Code (named profile, folder binding, or picker)",
		usage:   "claude-profile run [name] [--] [claude args...]",
		details: "Launches claude with the resolved profile's CLAUDE_CONFIG_DIR set for\n" +
			"that process only — nothing leaks into your shell session. The profile\n" +
			"is resolved in a strict order, and the result is always announced in a\n" +
			"banner — never a silent default:\n" +
			"\n" +
			"  1. A name given on the command line\n" +
			"  2. Otherwise, the nearest .claude-profile file (this directory,\n" +
			"     then its parents — like .git; see 'link')\n" +
			"  3. Otherwise, an interactive picker\n" +
			"\n" +
			"Everything after the name (or after --) is passed straight to claude.\n" +
			"A first argument that is not an existing profile is an error, never a\n" +
			"guess. The claude-<name> aliases created by 'create' call this command.\n" +
			"\n" +
			"run is the default action: 'claude-profile' with no subcommand, or with\n" +
			"a first argument that is not a subcommand, is the same as 'run'. So\n" +
			"'claude-profile devops' and the 'claudep' short alias launch profiles\n" +
			"directly.",
		examples: []string{
			"claude-profile run                        # binding, else picker",
			"claude-profile run devops                 # explicit",
			"claude-profile run go --continue          # extra args go to claude",
			"claude-profile run -- -p \"explain this\"   # no name, args to claude",
			"claude-profile devops                     # 'run' is optional",
		},
	},
	"link": {
		summary: "Bind the current directory to a profile",
		usage:   "claude-profile link <name>",
		details: "Writes a .claude-profile file into the current directory, so 'run'\n" +
			"without a name (and a guarded plain 'claude') here — and in any\n" +
			"subdirectory — uses this profile. The file is plain text; review it,\n" +
			"commit it to git, or delete it at any time.",
		examples: []string{
			"cd ~/work/acme && claude-profile link client-acme",
			"cd ~/infra/terraform && claude-profile link devops",
		},
	},
	"unlink": {
		summary: "Remove the current directory's binding",
		usage:   "claude-profile unlink",
		details: "Deletes the .claude-profile file in the current directory. If the\n" +
			"binding actually lives in a parent directory, unlink tells you where\n" +
			"instead of guessing.",
	},
	"status": {
		summary: "Show which profile would launch here, and why",
		usage:   "claude-profile status",
		details: "Answers \"where am I?\": the profile that 'run' would use in this\n" +
			"directory, how it was resolved, and its login state.",
	},
	"alias": {
		summary: "Set or change the short alias for claude-profile",
		usage:   "claude-profile alias [name] [--remove]",
		details: "The installer sets up 'claudep' as a short alias for claude-profile.\n" +
			"This command lets you rename it to anything you like, show the current\n" +
			"one, or remove it. The alias lives in the managed block of your shell\n" +
			"startup file; setting a new name replaces the old one.",
		options: [][2]string{
			{"--remove", "Remove the short alias from all shells."},
		},
		examples: []string{
			"claude-profile alias           # show the current alias",
			"claude-profile alias cpf       # rename claudep to cpf",
			"claude-profile alias --remove",
		},
	},
	"wrap": {
		summary: "Optionally wrap the 'claude' command itself",
		usage:   "claude-profile wrap <install|uninstall|status>",
		details: "Installs a tiny wrapper named 'claude' ahead of the real binary in\n" +
			"PATH, so profile resolution also applies outside your interactive\n" +
			"shell (IDE terminals, for example). Its contract is strict:\n" +
			"\n" +
			"  bound folder                → that profile, announced in a banner\n" +
			"  unbound + interactive       → the profile picker\n" +
			"  unbound + non-interactive   → the real claude, completely untouched\n" +
			"\n" +
			"Scripts, IDEs and CI in unbound folders therefore always get stock\n" +
			"claude behavior. 'wrap status' shows whether the wrapper is active\n" +
			"and which real binary it forwards to; 'wrap uninstall' removes it\n" +
			"cleanly. This is optional — the aliases installed by 'create' cover\n" +
			"interactive shells without wrapping anything.",
		examples: []string{
			"claude-profile wrap install",
			"claude-profile wrap status",
		},
	},
	"remove": {
		summary: "Delete a profile and its shell aliases",
		usage:   "claude-profile remove <name> [--force]",
		details: "Deletes the profile directory — including its login, settings and\n" +
			"history — and removes its aliases from your shell startup files.\n" +
			"Asks for confirmation unless --force is given.",
		options: [][2]string{
			{"--force", "Delete without confirmation (required when not running in a terminal)."},
		},
		examples: []string{
			"claude-profile remove scratch",
		},
	},
	"version": {
		summary: "Print the version",
		usage:   "claude-profile version",
	},
	"help": {
		summary: "Show help for a command",
		usage:   "claude-profile help [command]",
		examples: []string{
			"claude-profile help create",
		},
	},
}

// printUsage renders the top-level overview.
func (a *App) printUsage(w io.Writer) {
	st := a.Style
	fmt.Fprintf(w, "%s — isolated profiles for %s\n\n", st.bold("claude-profile"), st.cyan("Claude Code"))
	fmt.Fprintf(w, "Each profile has its own Claude account/login and its own toolkit —\n")
	fmt.Fprintf(w, "skills, agents, commands, settings, CLAUDE.md and history. One profile\n")
	fmt.Fprintf(w, "per subscription (company, client, personal), or lean per-stack setups.\n")
	fmt.Fprintf(w, "Every launch announces which profile is used and why — never a silent default.\n\n")
	fmt.Fprintf(w, "%s\n  claude-profile <command> [arguments]\n  claude-profile [profile] [claude args...]   %s\n\n", st.bold("Usage:"), st.dim("# 'run' is the default action"))
	fmt.Fprintf(w, "%s\n", st.bold("Commands:"))
	for _, name := range commandOrder {
		fmt.Fprintf(w, "  %s  %s\n", st.cyan(fmt.Sprintf("%-9s", name)), helpTopics[name].summary)
	}
	fmt.Fprintf(w, "\nRun %s for details on a command.\n\n", st.cyan("'claude-profile help <command>'"))
	fmt.Fprintf(w, "%s\n", st.bold("Environment:"))
	fmt.Fprintf(w, "  CLAUDE_PROFILES_DIR   Profile storage root (default: ~/.claude-profiles)\n")
	fmt.Fprintf(w, "  CLAUDE_PROFILE_RC     Force a specific shell startup file for aliases\n")
	fmt.Fprintf(w, "  NO_COLOR              Disable colored output\n")
}

// printHelp renders detailed help for one command.
func (a *App) printHelp(topic string) error {
	h, ok := helpTopics[topic]
	if !ok {
		return fmt.Errorf("unknown command %q — run 'claude-profile help' for the list", topic)
	}
	st := a.Style
	w := a.Stdout
	fmt.Fprintf(w, "%s — %s\n\n", st.bold("claude-profile "+topic), h.summary)
	fmt.Fprintf(w, "%s\n  %s\n", st.bold("Usage:"), h.usage)
	if h.details != "" {
		fmt.Fprintf(w, "\n%s\n", indent(h.details, "  "))
	}
	if len(h.options) > 0 {
		fmt.Fprintf(w, "\n%s\n", st.bold("Options:"))
		for _, opt := range h.options {
			fmt.Fprintf(w, "  %s\n      %s\n", st.cyan(opt[0]), opt[1])
		}
	}
	if len(h.examples) > 0 {
		fmt.Fprintf(w, "\n%s\n", st.bold("Examples:"))
		for _, ex := range h.examples {
			fmt.Fprintf(w, "  %s %s\n", st.dim("$"), ex)
		}
	}
	return nil
}

func indent(s, prefix string) string {
	out := prefix
	for _, r := range s {
		out += string(r)
		if r == '\n' {
			out += prefix
		}
	}
	return out
}
