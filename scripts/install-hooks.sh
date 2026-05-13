#!/usr/bin/env bash
# install-hooks.sh — install Forge's pre-commit hook (DEV-M1-36)
#
# Usage:
#   ./scripts/install-hooks.sh           install the pre-commit hook
#   ./scripts/install-hooks.sh --remove  uninstall the pre-commit hook

set -euo pipefail

ACTION="${1:-install}"
HOOK_SRC="$(cd "$(dirname "$0")" && pwd)/forge-pre-commit"
GIT_DIR="$(git rev-parse --git-dir 2>/dev/null)" || { echo "Error: not inside a git repository."; exit 1; }
HOOK_DEST="${GIT_DIR}/hooks/pre-commit"

case "$ACTION" in
  install)
    if [[ ! -f "$HOOK_SRC" ]]; then
      echo "Error: hook source not found at $HOOK_SRC"
      exit 1
    fi
    cp "$HOOK_SRC" "$HOOK_DEST"
    chmod +x "$HOOK_DEST"
    echo "Forge pre-commit hook installed at $HOOK_DEST"
    echo "  Runs: forge scan security --secrets && forge lint"
    echo "  Bypass: GITLEAKS_BYPASS_REASON=\"<reason>\" git commit ..."
    ;;
  --remove)
    if [[ -f "$HOOK_DEST" ]]; then
      rm "$HOOK_DEST"
      echo "Forge pre-commit hook removed from $HOOK_DEST"
    else
      echo "No pre-commit hook found at $HOOK_DEST"
    fi
    ;;
  *)
    echo "Usage: $0 [--remove]"
    exit 1
    ;;
esac
