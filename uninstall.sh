#!/bin/sh
# Uninstaller for claude-profile (macOS and Linux).
#
#   curl -fsSL https://raw.githubusercontent.com/victorRadu/claude-profile/main/uninstall.sh | sh
#
# Removes the binary, the wrapper shim and every managed shell-config line.
# Profile data (logins, history) is kept unless you confirm, or set
# CLAUDE_PROFILE_PURGE=1 for non-interactive removal.
#
# Environment:
#   CLAUDE_PROFILE_INSTALL_DIR  Where the binary was installed (default: ~/.local/bin)
#   CLAUDE_PROFILES_DIR         Profile storage root (default: ~/.claude-profiles)
#   CLAUDE_PROFILE_PURGE=1      Also delete all profile data without asking
set -eu

PROFILES_DIR="${CLAUDE_PROFILES_DIR:-$HOME/.claude-profiles}"
INSTALL_DIR="${CLAUDE_PROFILE_INSTALL_DIR:-$HOME/.local/bin}"
BLOCK_START="# >>> claude-profile >>>"
BLOCK_END="# <<< claude-profile <<<"

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-}" != "dumb" ]; then
  C_GREEN="$(printf '\033[32m')" C_RESET="$(printf '\033[0m')"
else
  C_GREEN="" C_RESET=""
fi
ok() { printf '%s✓%s %s\n' "$C_GREEN" "$C_RESET" "$*"; }

# --- managed shell-config blocks -----------------------------------------
remove_block() {
  file="$1"
  [ -f "$file" ] || return 0
  grep -qF "$BLOCK_START" "$file" || return 0
  tmp="$(mktemp)"
  awk -v s="$BLOCK_START" -v e="$BLOCK_END" '
    $0 == s { inblock=1; next }
    $0 == e { inblock=0; next }
    !inblock { print }
  ' "$file" > "$tmp" && cat "$tmp" > "$file" && rm -f "$tmp"
  ok "Removed managed block from $file"
}

remove_block "$HOME/.bashrc"
remove_block "$HOME/.bash_profile"
remove_block "${ZDOTDIR:-$HOME}/.zshrc"
remove_block "$HOME/.config/powershell/Microsoft.PowerShell_profile.ps1"
[ -n "${CLAUDE_PROFILE_RC:-}" ] && remove_block "$CLAUDE_PROFILE_RC"

# --- wrapper shim ----------------------------------------------------------
if [ -e "$PROFILES_DIR/.bin/claude" ]; then
  rm -f "$PROFILES_DIR/.bin/claude"
  rmdir "$PROFILES_DIR/.bin" 2>/dev/null || true
  ok "Removed the claude wrapper"
fi

# --- binary ----------------------------------------------------------------
if [ -e "$INSTALL_DIR/claude-profile" ]; then
  rm -f "$INSTALL_DIR/claude-profile"
  ok "Removed $INSTALL_DIR/claude-profile"
fi

# --- profile data (opt-in) ---------------------------------------------------
if [ -d "$PROFILES_DIR" ]; then
  purge="n"
  if [ "${CLAUDE_PROFILE_PURGE:-0}" = "1" ]; then
    purge="y"
  elif [ -t 0 ]; then
    printf 'Also delete all profile data in %s? This includes logins and history. [y/N] ' "$PROFILES_DIR"
    read -r purge || purge="n"
  fi
  case "$purge" in
    y|Y|yes)
      rm -rf "$PROFILES_DIR"
      ok "Deleted $PROFILES_DIR"
      ;;
    *)
      printf 'Kept profile data at %s — delete it manually when ready.\n' "$PROFILES_DIR"
      ;;
  esac
fi

printf '\nNote: .claude-profile binding files inside your project folders are not touched.\n'
printf 'claude-profile has been uninstalled. Open a new terminal to apply.\n'
