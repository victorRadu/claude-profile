# Update and migration strategy

How claude-profile keeps itself and existing profiles up to date. Three
independent pieces, each useful alone: a **version check** that tells users an
update exists, a **self-update** command that installs it, and a **migration
framework** that brings already-created profiles up to date with features new
profiles get automatically.

The design follows the project's core principle: nothing silent, nothing
hidden. Every automatic action is announced, cheap, and skippable.

## Version check

The foreground process never blocks on the network. Instead:

1. On startup of a user-facing command, claude-profile reads a small cache
   (`<profiles root>/.state/update.json`). Reading one file is the entire
   startup cost.
2. If the cache is older than 24 hours, it spawns itself detached
   (`claude-profile update --refresh-cache`, stdio to null) and moves on. The
   background process queries the GitHub releases API
   (`/repos/victorRadu/claude-profile/releases/latest`, 10s timeout) and
   rewrites the cache. Failures are silent — the cache timestamp still
   advances, so an offline machine probes at most once a day.
3. If the *cached* latest version is newer than the running binary, a single
   dim line goes to **stderr** (never stdout, so piped output stays clean):

   ```
   ↑ claude-profile 0.4.0 is available (you have 0.3.1) — run: claude-profile update
   ```

   The notice repeats at most once per 24 hours per version
   (`.state/notice.json`).

The check is skipped entirely when: `CLAUDE_PROFILE_NO_UPDATE_CHECK` is set,
`CI` is set, stdin is not a terminal, the build is a dev/local build, or the
command is machine-facing (`statusline`, `wrap-exec`, `update`, `migrate`,
`version`, `help`). Files under `.state/` can never collide with profiles:
profile names may not start with a dot.

## Self-update: `claude-profile update`

1. Query the releases API synchronously for the latest version. Already
   current → say so, done. (`--check` stops here; exit code 1 means an update
   exists, for scripts.)
2. Download the release asset for this OS/arch
   (`claude-profile_<ver>_<os>_<arch>.tar.gz`, zip on Windows) plus
   `checksums.txt`, and verify the asset's SHA-256 before touching anything.
3. Extract the binary to `<exe>.new` in the install directory and swap:
   - **Unix:** `chmod 0755` and an atomic `rename(2)` over the current path.
   - **Windows:** a running exe cannot be overwritten — rename the running
     binary to `<exe>.old`, move the new one into place, and delete the
     leftover `.old` best-effort on the next start.
   - No write permission (system-managed install): fail with the exact
     installer command to run instead. Never escalate privileges.
4. Run `<exe> migrate` as a child with inherited stdio — migrations always
   execute the **new** binary's code, and their output is part of the update
   transcript.

Dev builds (`version` = `dev` or `*-local`) refuse to self-update.

Trust model: HTTPS to github.com only, checksum from the same release. This
protects against corrupted or truncated downloads, not against a compromised
repository — the same trust already placed in `install.sh`.

## Profile migrations

New features often land in two places: `create` gives them to new profiles,
and a **migration** gives them to existing ones. Example: profiles created
before the status line feature have no statusLine in settings.json.

- Migrations live in an **append-only registry** (`internal/migrate`), each
  with a numeric ID, a name, and an idempotent `Apply(profile)` function.
  IDs never change meaning; new migrations only get appended.
- Every profile records what has been applied in
  `<profile>/.claude-profile-meta.json`. `create` stamps all current IDs at
  birth (the features are already built in), so migrations only ever run on
  profiles that predate them.
- A recorded migration is **never re-applied**. If a user deliberately
  removes a feature afterwards (e.g. `statusline uninstall`), no migration
  forces it back.
- Runner rules: apply pending migrations in ID order, per profile; announce
  every change (`✓ acme: status line shows profile and model`); a failure in
  one profile is reported, does not block others, and is retried next run
  (only success is recorded). Migrations obey the same hard rules as the rest
  of the code: never touch credentials or history, never overwrite foreign
  configuration (chain/stash instead).

### When migrations run

- **Automatically, announced** — on the first interactive run of a new binary
  version (tracked in `.state/migrated-version`), pending migrations run
  before the command, printing to stderr. Non-interactive invocations never
  auto-migrate; they retry on the next interactive one.
- **Explicitly** — `claude-profile migrate` applies pending migrations,
  `claude-profile migrate --status` shows the full matrix (profile ×
  migration → applied/pending) without changing anything.
- **After `update`** — step 4 above, so an update and its migrations happen
  as one visible transaction.

### Adding a migration (checklist for contributors)

1. Append to the registry in `internal/migrate/migrations.go` with the next
   ID. Never renumber or delete entries.
2. `Apply` must be idempotent, must skip cleanly when the feature is already
   present (or user-modified), and must not touch credentials/history.
3. Make `create` produce the same end state for new profiles.
4. Add tests: applies once, records, respects user removal, survives re-run.

## State files

| File | Writer | Contents |
|---|---|---|
| `.state/update.json` | background refresher | last check time, latest known version |
| `.state/notice.json` | foreground | last notice time + version |
| `.state/migrated-version` | foreground | binary version whose migrations last completed |
| `<profile>/.claude-profile-meta.json` | migrate runner / create | applied migration records |

Each file has exactly one writer to avoid write races. All are plain
JSON/text a user can read or delete; deleting any of them is safe (worst
case: one extra check, notice, or no-op migration pass).

## Environment

| Variable | Effect |
|---|---|
| `CLAUDE_PROFILE_NO_UPDATE_CHECK` | disables the background check and the notice (self-update stays available) |
| `CI` | same effect as above |
