# Contributing

Thanks for your interest in improving claude-profile!

## Getting started

Requirements: Go 1.24+ (no other dependencies). Older Go versions produce
macOS binaries without an `LC_UUID` load command, which macOS 15 refuses to
run (golang/go#68678) — 1.24 is the floor for that reason, not for language
features.

```sh
git clone https://github.com/victorRadu/claude-profile
cd claude-profile
make          # lint + test + build
make hooks    # optional: lint on commit, tests on push
```

To try your local build with the full install experience (aliases included), run `./install.sh --local` (macOS/Linux) or `.\install.ps1 -Local` (Windows). Remove everything again with `./uninstall.sh`.

## Development guidelines

- **Zero external dependencies.** The tool is stdlib-only by design — it keeps the binary small, the supply chain empty, and the code auditable. PRs adding dependencies need a very strong reason.
- **Never touch user data outside the managed block.** All shell startup file edits must go through `internal/shell` and stay between the `# >>> claude-profile >>>` / `# <<< claude-profile <<<` markers.
- **Never copy credentials or history** between profiles. The copy allow-list lives in `internal/profile`.
- **All three platforms matter.** Code must build and behave sensibly on macOS, Linux and Windows; CI enforces this. Use build tags for platform-specific behaviour (see `internal/cli/launch_*.go`).
- **Tests accompany behaviour.** Every command and every rc-file edge case has a test; keep it that way. Run `go test -race ./...`.

## Project layout

```
main.go                  entry point (version injected via ldflags)
internal/cli/            command dispatch, flags, prompts, launching
internal/profile/        profile store: create/list/remove/copy
internal/shell/          shell detection + managed rc-file blocks
```

## Submitting changes

1. Fork and create a feature branch.
2. `make` must pass (gofmt, go vet, tests).
3. Use clear commit messages (`fix:`, `feat:`, `docs:`, ... prefixes appreciated — they feed the release changelog).
4. Open a PR describing *why*, not just *what*.

## Releases (maintainers)

Tag `vX.Y.Z` on `main` and push it. GitHub Actions runs the test suite and GoReleaser publishes signed archives with checksums for all platforms.

## Reporting issues

Include your OS, shell, `claude-profile version` output, and the contents of the managed block from your shell startup file if the issue involves aliases.
