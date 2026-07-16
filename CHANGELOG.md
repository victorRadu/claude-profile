# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- `statusline` command: every profile's Claude Code status line shows the active profile and current model (`acme · Opus 4.8`). Installed automatically by `create`; an existing status line from another tool is chained after it, never overwritten, and restored exactly by `statusline uninstall`.
- `update` command: self-update from GitHub releases with SHA-256 verification, plus a daily background version check with a one-line notice (disable with `CLAUDE_PROFILE_NO_UPDATE_CHECK=1`).
- `migrate` command and append-only migration framework: profiles created by older versions pick up new features automatically — announced, idempotent, recorded per profile, and never re-applied once a user removes a feature (`migrate --status` shows the matrix). See `docs/updates.md`.

- Initial release: `create`, `list`, `run`, `link`, `unlink`, `status`, `remove`, `version` commands.
- Single launch command with deterministic resolution: `run <name>` (explicit) → `run` (`.claude-profile` directory binding) → interactive picker; never a silent default.
- Loud launch banner announcing the resolved profile and why it was chosen.
- Isolated profiles via `CLAUDE_CONFIG_DIR` under `~/.claude-profiles`.
- Shell integration for bash, zsh and PowerShell through a managed rc-file block.
- Optional guard alias redirecting plain `claude` to the profile picker.
- Optional `wrap` command: a PATH shim making plain `claude` profile-aware everywhere, with transparent pass-through to the real binary for scripts and IDEs in unbound directories.
- Profile cloning with `--from` (credentials and history are never copied).
- Cross-platform binaries (macOS, Linux, Windows) with checksummed releases.
- One-line uninstall scripts (`uninstall.sh`, `uninstall.ps1`) that remove the binary, wrapper and managed shell-config lines while keeping profile data unless explicitly purged.
- `alias` command managing a short alias for the tool itself (installers set up `claudep`; rename with `claude-profile alias <name>`).
