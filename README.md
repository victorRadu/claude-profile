# claude-profile

**One machine, multiple Claude accounts.** Claude Code supports exactly one login — claude-profile gives you one per project: your company seat, your client's seat, your personal Max plan. Switch by running a different command, never by logging out and back in.

[![CI](https://github.com/victorRadu/claude-profile/actions/workflows/ci.yml/badge.svg)](https://github.com/victorRadu/claude-profile/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/victorRadu/claude-profile)](https://github.com/victorRadu/claude-profile/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

```
$ claude-profile create client-acme
✓ Created profile client-acme (~/.claude-profiles/client-acme)
✓ Added zsh alias claude-client-acme (~/.zshrc)

$ claude-client-acme          # log in once with the client's account
$ claude-personal             # your own subscription, untouched
```

If you work with more than one Claude subscription — consulting for clients who provide a seat, a company account plus a personal plan, separate billing per project — Claude Code makes you `/logout` and `/login` every time you switch, and everything else (settings, skills, history) is shared between accounts anyway.

claude-profile gives every account its own complete, isolated profile: login, settings, skills, agents, `CLAUDE.md` and history. Usage lands on the right subscription, client conversations never mix with personal ones, and each rate limit is its own.

**The same isolation also cuts waste.** A profile only loads what's in it — so a lean `go` or `devops` profile with just that stack's skills and conventions means less context pollution and lower token usage than one config carrying everything you've ever installed. Great for scratch profiles to test new skill packs, too.

## Install

**macOS / Linux**

```sh
curl -fsSL https://raw.githubusercontent.com/victorRadu/claude-profile/main/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/victorRadu/claude-profile/main/install.ps1 | iex
```

That's it — a single small executable, nothing else to install. The installer also sets up `claudep` as a short alias, so `claudep status` works everywhere `claude-profile status` does — and `claudep client-acme` (or bare `claudep`) launches a profile, since `run` is the default action. It's yours to change: `claude-profile alias cpf` renames it, `claude-profile alias --remove` drops it. (Binaries for every platform are also on the [releases page](https://github.com/victorRadu/claude-profile/releases/latest).)

## Get started

```sh
claude-profile create client-acme --from default   # seed with your current ~/.claude setup
claude-profile create personal                     # or start from a clean slate
```

Open a new terminal and run `claude-client-acme` — then `/login` once with the client's account. That login, and everything that happens under it, now lives only in that profile. `claude-personal` is your own subscription, with its own history and rate limits.

`--from` copies settings, `CLAUDE.md`, skills, agents and commands from your existing setup (or any other profile) as a starting point. Credentials and history are never copied — a new profile always starts with a clean login.

## You always know which profile you're in

claude-profile never keeps a hidden "active profile" and never falls back to a silent default. Every launch prints a banner first:

```
◆ profile: client-acme (~/.claude-profiles/client-acme) — via .claude-profile in ~/work/acme
```

That matters most when accounts are involved: billing a client's project to your personal plan — or leaking a personal experiment into a client's workspace — is exactly the mistake this design makes impossible to miss. If you let claude-profile guard the plain `claude` command (offered during `create`), even typing `claude` out of habit can never silently land in the wrong account — it uses the current folder's binding or asks you to pick.

The reminder doesn't stop at launch. Every new profile also gets a Claude Code status line showing the profile and the current model for the whole session:

```
client-acme · Opus 4.8
```

If a profile already has a status line from another tool, it is never overwritten — its output is kept and shown right after the profile and model, and `claude-profile statusline uninstall <name>` restores it exactly as it was.

## Bind a folder to a profile

Profiles usually follow projects, so bind a project folder once:

```sh
cd ~/work/acme
claude-profile link client-acme
```

From now on, `claude-profile run` (or a guarded plain `claude`) anywhere inside that folder uses the client's account — announced in the banner every time. The binding is a plain-text `.claude-profile` file you can read, delete, or commit to git so your whole team gets the same profile name.

Not sure where you are? Ask:

```
$ claude-profile status
Profile root:   ~/.claude-profiles
This directory: "client-acme" via .claude-profile in ~/work/acme
Login:          logged in
```

## Optional: wrap the `claude` command everywhere

The guard alias works in your interactive shell. If you want profile resolution to also apply where aliases don't reach — IDE terminals, for example — you can install a real wrapper:

```sh
claude-profile wrap install
```

This puts a tiny `claude` shim ahead of the real binary in PATH, with a strict contract:

| Situation | Behavior |
|---|---|
| Bound folder | That profile, announced in a banner |
| Unbound + interactive terminal | The profile picker |
| Unbound + script / IDE / CI | The **real claude, completely untouched** |

The last row is the compatibility guarantee: anything invoking `claude` programmatically in an unbound folder gets byte-identical stock behavior. `claude-profile wrap status` shows whether the wrapper is active and which real binary it forwards to; `claude-profile wrap uninstall` removes it cleanly. This is entirely optional — nothing installs it by default.

## Commands

```
create      Create a new profile
list        List profiles and their login state
run         Launch Claude Code — named profile, folder binding, or picker
link        Bind the current folder to a profile
unlink      Remove the current folder's binding
status      Show which profile would launch here, and why
statusline  Show profile name and model in Claude Code's status line
alias       Set or change the short alias for claude-profile
wrap        Optionally wrap the claude command itself
remove      Delete a profile and its shell aliases
update      Update claude-profile to the latest release
migrate     Bring existing profiles up to date with new features
```

One launch command, three tiers:

```sh
claude-profile run client-acme   # explicit — this profile, now
claude-profile run               # this folder's binding, else the picker
claude-profile run -- -p "hi"    # after --, everything goes to claude
```

`run` is the default action, so you can drop the word entirely — anything that
isn't a subcommand is treated as a launch. This is what makes the short alias
so quick:

```sh
claudep client-acme              # same as: claude-profile run client-acme
claudep                          # this folder's binding, else the picker
claudep -- -p "hi"               # after --, everything goes to claude
```

Every command has detailed built-in help: `claude-profile help <command>`.

Useful extras: `create go --from php` clones settings, skills and agents from an existing profile (credentials and history are never copied); `run client-acme --continue` passes everything after the name straight to Claude Code.

## Staying up to date

At most once a day, claude-profile checks in the background whether a newer release exists (the foreground never waits on the network) and prints a one-line notice when one does:

```
↑ claude-profile 0.4.0 is available (you have 0.3.1) — run: claude-profile update
```

`claude-profile update` downloads the release, verifies its checksum, replaces the binary in place, and finishes by running `migrate`, which brings profiles created by older versions up to date with features new profiles get automatically (like the status line). Migrations are never silent, never touch credentials or history, never overwrite configuration from other tools, and never force back a feature you deliberately removed. `claude-profile migrate --status` shows exactly where every profile stands; `CLAUDE_PROFILE_NO_UPDATE_CHECK=1` turns the daily check off. Details in [docs/updates.md](docs/updates.md).

## Good to know

- **Your shell config is safe.** Aliases live between two clearly marked lines in your shell startup file; nothing outside them is ever touched. An existing `claude` alias of your own is never overridden.
- **Profiles are just folders** under `~/.claude-profiles`. Nothing is hidden anywhere else.
- **Uninstall as easily as you installed** — `curl -fsSL https://raw.githubusercontent.com/victorRadu/claude-profile/main/uninstall.sh | sh` (or `uninstall.ps1` on Windows) removes the binary, the wrapper and every managed shell-config line. Your profile data (logins, history) is kept unless you explicitly confirm deleting it.
- **Colors** follow your terminal and can be disabled with `NO_COLOR=1`.

## Platform support

| OS | Terminals |
|---|---|
| macOS (Intel & Apple Silicon) | Terminal, iTerm2, anything running bash/zsh, PowerShell |
| Linux (amd64 & arm64) | bash, zsh, PowerShell |
| Windows 10+ | PowerShell, Windows Terminal |

## Contributing

Bug reports and pull requests are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
