#!/usr/bin/env bash
# scripts/npm-publish.sh
#
# Stamps package.json versions and copies goreleaser binaries into each
# platform package's bin/ directory, ready for `npm publish`.
#
# Usage:
#   bash scripts/npm-publish.sh --version 1.2.3 [--dry-run true|false]
#
# Expected goreleaser dist/ layout (produced by .goreleaser.yml):
#   dist/forge_linux_amd64/forge
#   dist/forge_linux_arm64/forge
#   dist/forge_darwin_amd64/forge
#   dist/forge_darwin_arm64/forge
#   dist/forge_windows_amd64/forge.exe

set -euo pipefail

# ── Argument parsing ─────────────────────────────────────────────────────────

VERSION=""
DRY_RUN="true"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)  VERSION="$2";  shift 2 ;;
    --dry-run)  DRY_RUN="$2";  shift 2 ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

if [[ -z "$VERSION" ]]; then
  echo "ERROR: --version is required" >&2
  exit 1
fi

echo "npm-publish.sh  version=${VERSION}  dry-run=${DRY_RUN}"

# ── Platform map: <npm-pkg-dir>  <goreleaser-dist-subdir>  <binary-name> ─────

declare -A PLATFORMS=(
  ["cli-linux-x64"]="forge_linux_amd64/forge forge"
  ["cli-linux-arm64"]="forge_linux_arm64/forge forge"
  ["cli-darwin-x64"]="forge_darwin_amd64/forge forge"
  ["cli-darwin-arm64"]="forge_darwin_arm64/forge forge"
  ["cli-win32-x64"]="forge_windows_amd64/forge.exe forge.exe"
)

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# ── Process each platform package ────────────────────────────────────────────

for pkg_dir in "${!PLATFORMS[@]}"; do
  read -r src_rel bin_name <<< "${PLATFORMS[$pkg_dir]}"

  pkg_path="${REPO_ROOT}/packages/${pkg_dir}"
  src_path="${REPO_ROOT}/dist/${src_rel}"
  dest_path="${pkg_path}/bin/${bin_name}"

  echo ""
  echo "── ${pkg_dir} ──"
  echo "   src:  ${src_path}"
  echo "   dest: ${dest_path}"

  if [[ ! -f "$src_path" ]]; then
    echo "   WARN: source binary not found — skipping (expected in CI from goreleaser)" >&2
    continue
  fi

  mkdir -p "${pkg_path}/bin"
  cp "${src_path}" "${dest_path}"
  chmod +x "${dest_path}"

  # Stamp version in package.json
  if command -v jq &>/dev/null; then
    tmp=$(mktemp)
    jq --arg v "$VERSION" '.version = $v | .optionalDependencies //= {} | .peerDependencies //= {}' \
      "${pkg_path}/package.json" > "$tmp" && mv "$tmp" "${pkg_path}/package.json"
  else
    # Fallback: sed — less robust but zero extra deps
    sed -i.bak "s/\"version\": \"[^\"]*\"/\"version\": \"${VERSION}\"/" "${pkg_path}/package.json"
    rm -f "${pkg_path}/package.json.bak"
  fi

  echo "   version stamped: ${VERSION}"
done

# ── Stamp @forge/cli wrapper version + optionalDependencies versions ─────────

wrapper_pkg="${REPO_ROOT}/packages/cli/package.json"
echo ""
echo "── @forge/cli wrapper ──"

if command -v jq &>/dev/null; then
  tmp=$(mktemp)
  jq --arg v "$VERSION" '
    .version = $v |
    .optionalDependencies = (
      .optionalDependencies | to_entries |
      map(.value = $v) | from_entries
    )
  ' "$wrapper_pkg" > "$tmp" && mv "$tmp" "$wrapper_pkg"
else
  sed -i.bak "s/\"version\": \"[^\"]*\"/\"version\": \"${VERSION}\"/g" "$wrapper_pkg"
  rm -f "${wrapper_pkg}.bak"
fi

echo "   version stamped: ${VERSION}"
echo ""

if [[ "$DRY_RUN" == "true" ]]; then
  echo "DRY-RUN: no packages published."
  echo "Run with --dry-run false to publish."
else
  echo "Ready for npm publish (called by CI workflow)."
fi
