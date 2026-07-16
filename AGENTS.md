# AGENTS.md

Guidance for AI agents working in this repository. Human contributors should read `CONTRIBUTING.md`; this file distills the parts an agent needs to make correct changes fast.

## What this project is

`claude-profile` is a single, dependency-free Go CLI that gives Claude Code one isolated profile per project (separate login, settings, skills, agents, history). It works by pointing Claude Code at a per-profile `CLAUDE_CONFIG_DIR` and by writing launcher aliases into the user's shell startup file. See `README.md` for the user-facing story.

## Build, test, lint

Requires Go 1.22+ and nothing else (stdlib only). Use the Makefile:

- `make` — lint + test + build (run this before proposing a change is done)
- `make build` — compile to `./claude-profile` (version injected via ldflags)
- `make test` — `go test -race ./...`
- `make lint` — `go vet`, `gofmt -s` check, and `golangci-lint` if installed
- `make fmt` — apply `gofmt -s -w .`
- `make cross` — build all release platforms into `dist/`

CI (`.github/workflows/ci.yml`) runs build + test on macOS, Linux and Windows, plus `golangci-lint` (config in `.golangci.yml`). All three platforms must pass.

## Project layout

```
main.go                  entry point; version set via -ldflags, calls cli.Run
internal/cli/            command dispatch, flag parsing, prompts, launching
internal/profile/        profile store: create/list/remove/copy, name validation
internal/shell/          shell detection + managed rc-file block editing
install.sh / .ps1        installers (download release or --local build)
uninstall.sh / .ps1      removers
.goreleaser.yaml         release build/publish config (GoReleaser v2)
.github/workflows/       ci.yml (push/PR) and release.yml (tag push)
```

Command implementations live in `internal/cli/commands.go` and `resolve_commands.go`. Platform-specific behaviour uses build tags — see `launch_unix.go` / `launch_windows.go`, `terminal_*.go`, `ui_unix.go` / `ui_windows.go`. `App` (in `cli.go`) carries all dependencies (streams, store, launcher) so commands are testable with substituted I/O.

## Hard rules (do not violate)

These are load-bearing invariants of the project, not style preferences:

- **Zero external dependencies.** Stdlib only. Do not add anything to `go.mod`. Adding a dependency needs an exceptional reason and explicit maintainer sign-off.
- **Never touch user data outside the managed block.** All shell startup file edits go through `internal/shell` and stay strictly between the `# >>> claude-profile >>>` and `# <<< claude-profile <<<` markers. Nothing outside those markers may ever be modified.
- **Never copy credentials or history between profiles.** The copy allow-list is `copyItems` in `internal/profile/profile.go` (`settings.json`, `CLAUDE.md`, `skills`, `agents`, `commands`). Do not add credential, history, or cache entries.
- **Profile resolution is always explicit and always announced.** No hidden "active profile", no silent default fallback. Every launch prints the profile banner first. Features that add hidden state break the core design principle.
- **All three platforms matter.** Code must build and behave sensibly on macOS, Linux and Windows. Use build tags for platform-specific paths.
- **Tests accompany behaviour.** Every command and every rc-file edge case has a test (`*_test.go` alongside each package). Add or update tests with any behavioural change.

## Conventions

- Formatting: `gofmt -s` (enforced). Run `make fmt` before committing.
- `fmt.Fprint*` writes to the CLI's own stdout/stderr are exempt from errcheck (see `.golangci.yml`); everything else must check errors.
- Commit messages use conventional prefixes (`feat:`, `fix:`, `docs:`, `ci:`, `chore:`) — they feed the release changelog (`.goreleaser.yaml` changelog filters).
- Optional git hooks: `make hooks` enables lint-on-commit and test-on-push (`.githooks/`).

## Releasing (maintainers)

Merging to `main` only runs CI — it does **not** publish a release. A release is published only when a `vX.Y.Z` tag is pushed, which triggers `.github/workflows/release.yml` → GoReleaser. The tag's leading `v` is stripped for asset names (tag `v0.1.0` → `claude-profile_0.1.0_<os>_<arch>.tar.gz`), matching what `install.sh` expects. If `install.sh` reports "could not determine the latest release", it usually means no release has been published for the tag yet.
