#!/bin/sh
# Installer for claude-profile (macOS and Linux).
#
#   curl -fsSL https://raw.githubusercontent.com/victorRadu/claude-profile/main/install.sh | sh
#
# From a checkout, contributors can install their local build instead:
#
#   ./install.sh --local
#
# Environment:
#   CLAUDE_PROFILE_INSTALL_DIR  Install destination (default: ~/.local/bin)
#   CLAUDE_PROFILE_VERSION      Version to install (default: latest)
set -eu

REPO="victorRadu/claude-profile"
INSTALL_DIR="${CLAUDE_PROFILE_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${CLAUDE_PROFILE_VERSION:-latest}"
LOCAL=0

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-}" != "dumb" ]; then
  C_GREEN="$(printf '\033[32m')" C_BOLD="$(printf '\033[1m')" C_DIM="$(printf '\033[2m')" C_RESET="$(printf '\033[0m')"
else
  C_GREEN="" C_BOLD="" C_DIM="" C_RESET=""
fi

err() { printf 'Error: %s\n' "$*" >&2; exit 1; }
ok()  { printf '%s✓%s %s\n' "$C_GREEN" "$C_RESET" "$*"; }

for arg in "$@"; do
  case "$arg" in
    --local) LOCAL=1 ;;
    -h|--help) sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) err "unknown option '$arg' (try --local or --help)" ;;
  esac
done

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

if [ "$LOCAL" = "1" ]; then
  # --- build from this checkout ----------------------------------------------
  src_dir="$(cd "$(dirname "$0")" && pwd)"
  [ -f "$src_dir/main.go" ] || err "--local must be run from a claude-profile checkout"
  command -v go >/dev/null 2>&1 || err "--local requires the Go toolchain (https://go.dev/dl)"

  version="$(git -C "$src_dir" describe --tags --always 2>/dev/null || echo dev)"
  printf 'Building claude-profile %s-local from %s...\n' "$version" "$src_dir"
  (cd "$src_dir" && go build -ldflags "-s -w -X main.version=${version}-local" -o "$tmp/claude-profile" .)
else
  # --- download a release ------------------------------------------------------
  case "$(uname -s)" in
    Darwin) os="darwin" ;;
    Linux)  os="linux" ;;
    *) err "unsupported OS '$(uname -s)'. On Windows, use install.ps1 or download from GitHub Releases." ;;
  esac

  case "$(uname -m)" in
    x86_64|amd64)  arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) err "unsupported architecture '$(uname -m)'" ;;
  esac

  command -v curl >/dev/null 2>&1 || err "curl is required"
  command -v tar  >/dev/null 2>&1 || err "tar is required"

  if [ "$VERSION" = "latest" ]; then
    VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
      | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')"
    [ -n "$VERSION" ] || err "could not determine the latest release"
  fi
  version_num="${VERSION#v}"

  asset="claude-profile_${version_num}_${os}_${arch}.tar.gz"
  base="https://github.com/$REPO/releases/download/$VERSION"

  printf 'Downloading claude-profile %s (%s/%s)...\n' "$VERSION" "$os" "$arch"
  curl -fsSL -o "$tmp/$asset" "$base/$asset" || err "download failed: $base/$asset"
  curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || err "checksum download failed"

  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$tmp" && grep " $asset\$" checksums.txt | sha256sum -c - >/dev/null) \
      || err "checksum verification failed"
  elif command -v shasum >/dev/null 2>&1; then
    (cd "$tmp" && grep " $asset\$" checksums.txt | shasum -a 256 -c - >/dev/null) \
      || err "checksum verification failed"
  else
    printf 'Warning: sha256sum/shasum not found, skipping checksum verification.\n' >&2
  fi

  tar -xzf "$tmp/$asset" -C "$tmp"
fi

# --- install -------------------------------------------------------------------
mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/claude-profile" "$INSTALL_DIR/claude-profile"

# Short shell alias, managed by the tool itself (rename: claude-profile alias <name>)
"$INSTALL_DIR/claude-profile" alias claudep >/dev/null 2>&1 || true

ok "Installed $INSTALL_DIR/claude-profile"
ok "Short alias: claudep ${C_DIM}(rename with: claude-profile alias <name>)${C_RESET}"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) printf '\nNote: %s is not in your PATH. Add this to your shell profile:\n' "$INSTALL_DIR"
     printf '  export PATH="%s:$PATH"\n' "$INSTALL_DIR" ;;
esac

printf '\nGet started:\n  %sclaude-profile create frontend --from default%s\n' "$C_BOLD" "$C_RESET"
