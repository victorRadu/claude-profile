# Profile copy

How copying configuration into a new profile works, and why it works the way
it does. Copying happens in one place: the `create` command, either through
`--from <profile|default>` or through the interactive prompt when neither is
given.

The design follows the project's core principle: nothing silent, nothing
hidden. The user sees what will be copied, chooses how much of it they want,
and two things are excluded unconditionally — credentials and history.

## What "copy" means

A profile is more than its files. Claude Code keeps per-profile state in two
places:

- **Files in the config directory** — `settings.json`, `CLAUDE.md`, and the
  `skills/`, `agents/` and `commands/` directories.
- **The state file `.claude.json`** — first-run/onboarding flags, UI
  preferences, and a `projects` map holding per-folder trust
  (`hasTrustDialogAccepted`), tool permissions (`allowedTools`), CLAUDE.md
  external-includes approval, and MCP server configuration.

Earlier versions copied only the files, which is why a copied profile re-ran
the init dialog and re-asked for every folder approval: all of that lives in
the state file. A copy now carries both, filtered as described below.

## The interactive choice

When creating a profile interactively, after picking a source the user gets a
three-way choice:

```
Copy configuration from 'work' into the new profile?
❯ Copy everything (all shareable config — credentials and history are never copied)
  Choose what to copy
  Start clean — don't copy anything
```

**Copy everything** takes all shareable configuration. **Choose what to
copy** walks the categories below — only those actually present in the
source — with incremental control: simple categories are yes/no, collection
categories offer *copy all / choose individually / skip*, and "choose" opens
a multi-select list of the individual items.

| Category | Granularity |
|---|---|
| Settings (`settings.json`) | yes/no |
| CLAUDE.md | yes/no |
| Skills | all / individual skills |
| Agents | all / individual agents |
| Commands | all / individual commands |
| Preferences & onboarding (skips first-run setup) | yes/no |
| Folder trust & permissions | all / individual folders |
| MCP servers | all / individual servers |

When stdin is not a terminal (scripts, CI), `--from` copies everything —
the most useful default, and the only honest one, since nobody is there to
choose.

## What is never copied, and why

Regardless of mode, the copy strips:

- `oauthAccount` and `userID` (state file, top level) — account identity.
  A copied profile gets its own identity on first login.
- `history` (state file, per project) and any history/session files —
  conversations belong to the profile they happened in.
- `lastSessionId` (state file, per project) — sessions are not copied, so
  a carried-over ID would dangle and could confuse `claude --continue`.
- Credentials (`.credentials.json` on Linux; the OS keychain elsewhere) —
  profiles exist to isolate logins. A copied profile always starts with
  `/login`.
- Symlinks inside copied directories — they may point at credentials or at
  files outside the profile, and following them would break isolation.

This is the deliberate gap between "identical" and "shareable": a copy
behaves like the original (no re-onboarding, no re-approving folders) but is
never *logged in* as the original.

## Why a deny-list for the state file

The state file is filtered by removing known-sensitive keys rather than by
keeping known-good ones. Claude Code adds new configuration and approval
flags far more often than it adds sensitive ones — credentials deliberately
live outside `.claude.json` — and an allow-list would silently drop every
new flag, making re-prompts creep back into copies as Claude Code evolves.
The deny-list fails in the harmless direction: at worst a copy carries a
stale cache value, not a missing approval.

The denied keys are the floor, not an option: no mode and no interactive
choice can include them.

## The default source quirk

When copying from `default` (the unmanaged `~/.claude` setup), the state
file is `~/.claude.json` — Claude Code keeps it in the home directory, not
inside `~/.claude`. Old `~/.claude.json` files also contain per-project
`history` arrays from before history moved out of the state file; the
deny-list strips those like anything else.

A missing or unparseable state file is tolerated: the copy proceeds with
files only, matching how a missing `skills/` directory is simply skipped.

## Where this lives in the code

`internal/profile/copy.go` owns the mechanics: the `Selection` type (which
categories, all-or-named-items), `TakeInventory` (what exists in a source,
for the prompts), `CopyFrom` (files + filtered state file), and the deny
lists. `internal/cli` owns the prompts and builds the `Selection`; it never
filters anything itself. All of it is stdlib only.
