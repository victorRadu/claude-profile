# Security Policy

## What claude-profile touches

claude-profile is designed to have a small, auditable footprint:

- It creates and deletes directories only under the profile root (`~/.claude-profiles` by default).
- It edits shell startup files only between the `# >>> claude-profile >>>` and `# <<< claude-profile <<<` markers.
- It never reads, copies or transmits credentials: profile cloning explicitly excludes `.credentials.json` and history, and symlinks are not followed when copying.
- It has zero third-party dependencies and makes no network requests.

## Reporting a vulnerability

Please report suspected vulnerabilities privately via [GitHub Security Advisories](https://github.com/victorRadu/claude-profile/security/advisories/new) rather than opening a public issue. Include the version (`claude-profile version`), your OS, and reproduction steps. You can expect an initial response within a few days.
