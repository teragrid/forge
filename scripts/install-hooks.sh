#!/usr/bin/env bash
# install-hooks.sh — install Forge's git hooks (DEV-M1-36, DEV-M3-30, DEV-M3-31)
#
# Usage:
#   ./scripts/install-hooks.sh           install the pre-commit hook (legacy default)
#   ./scripts/install-hooks.sh --all     install pre-commit + pre-push + post-push
#   ./scripts/install-hooks.sh --remove  uninstall the pre-commit hook
#   ./scripts/install-hooks.sh --remove-all  uninstall all three hooks
#
# Note: `make hooks` (recommended) uses `git config core.hooksPath .githooks`
# which activates .githooks/ automatically for all three hooks without copying.
# Use this script only when you prefer the .git/hooks/ copy approach.

set -euo pipefail

ACTION="${1:-install}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GIT_DIR="$(git rev-parse --git-dir 2>/dev/null)" || { echo "Error: not inside a git repository."; exit 1; }
HOOKS_DIR="${GIT_DIR}/hooks"

# Source files
PRE_COMMIT_SRC="${SCRIPT_DIR}/forge-pre-commit"
PRE_PUSH_SRC="${SCRIPT_DIR}/../.githooks/pre-push"
POST_PUSH_SRC="${SCRIPT_DIR}/../.githooks/post-push"

install_hook() {
  local src="$1" dest="$2" name="$3"
  if [[ ! -f "$src" ]]; then
    echo "Error: hook source not found at $src"
    exit 1
  fi
  cp "$src" "$dest"
  chmod +x "$dest"
  echo "Forge ${name} hook installed at ${dest}"
}

remove_hook() {
  local dest="$1" name="$2"
  if [[ -f "$dest" ]]; then
    rm "$dest"
    echo "Forge ${name} hook removed from ${dest}"
  else
    echo "No ${name} hook found at ${dest}"
  fi
}

case "$ACTION" in
  install)
    install_hook "$PRE_COMMIT_SRC" "${HOOKS_DIR}/pre-commit" "pre-commit"
    echo "  Runs: forge scan security --secrets && forge lint"
    echo "  Bypass: GITLEAKS_BYPASS_REASON=\"<reason>\" git commit ..."
    ;;
  --all)
    install_hook "$PRE_COMMIT_SRC" "${HOOKS_DIR}/pre-commit" "pre-commit"
    install_hook "$PRE_PUSH_SRC"   "${HOOKS_DIR}/pre-push"   "pre-push"
    install_hook "$POST_PUSH_SRC"  "${HOOKS_DIR}/post-push"  "post-push"
    echo ""
    echo "All Forge git hooks installed."
    echo "  pre-commit  — secret scan + convention lint (blocks commit)"
    echo "  pre-push    — fmt + vet + lint + build + test + vuln + forge gates + qa suite (blocks push)"
    echo "  post-push   — CI monitor; records gotchas to .forge/learned/ (non-blocking)"
    echo ""
    echo "  Recommended alternative: make hooks  (uses .githooks/ directly, no copy)"
    ;;
  --remove)
    remove_hook "${HOOKS_DIR}/pre-commit" "pre-commit"
    ;;
  --remove-all)
    remove_hook "${HOOKS_DIR}/pre-commit" "pre-commit"
    remove_hook "${HOOKS_DIR}/pre-push"   "pre-push"
    remove_hook "${HOOKS_DIR}/post-push"  "post-push"
    ;;
  *)
    echo "Usage: $0 [install|--all|--remove|--remove-all]"
    exit 1
    ;;
esac
