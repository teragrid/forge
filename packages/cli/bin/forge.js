#!/usr/bin/env node
// bin/forge.js — @forgeone/cli platform wrapper
//
// Resolves the pre-compiled Go binary for the current platform from the
// matching optional dependency package, then execs it transparently.
//
// Pattern used by: esbuild, @biomejs/biome, turbo, prisma, oxc-parser.
// No Go installation required on the user's machine.

"use strict";

const { execFileSync } = require("child_process");
const path = require("path");
const fs = require("fs");

// ── Platform → optional-dependency mapping ──────────────────────────────────

const PLATFORM_PACKAGES = {
  "linux-x64":    ["@forgeone/cli-linux-x64",    "forge"],
  "linux-arm64":  ["@forgeone/cli-linux-arm64",  "forge"],
  "darwin-x64":   ["@forgeone/cli-darwin-x64",   "forge"],
  "darwin-arm64": ["@forgeone/cli-darwin-arm64",  "forge"],
  "win32-x64":    ["@forgeone/cli-win32-x64",    "forge.exe"],
};

// ── Resolve binary path ──────────────────────────────────────────────────────

function resolveBinary() {
  const platform = `${process.platform}-${process.arch}`;
  const entry = PLATFORM_PACKAGES[platform];

  if (!entry) {
    fatal(
      `Unsupported platform: ${platform}\n` +
      `Supported: ${Object.keys(PLATFORM_PACKAGES).join(", ")}\n` +
      `You can build from source: https://github.com/teragrid/forge#building-from-source`
    );
  }

  const [pkgName, binaryName] = entry;

  // Strategy 1: Look alongside node.exe (npm global prefix).
  // process.execPath = "C:\Program Files\nodejs\node.exe" (or nvm equiv).
  // Global node_modules live at dirname(execPath)/node_modules on Windows,
  // and at dirname(dirname(execPath))/lib/node_modules on Unix/macOS.
  // This is robust against junction/symlink chains that confuse __dirname.
  const nodeDir = path.dirname(process.execPath);
  const globalCandidates = [
    path.join(nodeDir, "node_modules", pkgName, "bin", binaryName),           // Windows
    path.join(nodeDir, "..", "lib", "node_modules", pkgName, "bin", binaryName), // Unix/macOS
  ];
  for (const candidate of globalCandidates) {
    if (fs.existsSync(candidate)) return candidate;
  }

  // Strategy 2: require.resolve — handles standard installs and NODE_PATH setups.
  try {
    const pkgJsonPath = require.resolve(`${pkgName}/package.json`);
    const candidate = path.join(path.dirname(pkgJsonPath), "bin", binaryName);
    if (fs.existsSync(candidate)) return candidate;
  } catch (_) {
    // package not on require path — fall through to directory walk
  }

  // Strategy 3: Walk up from the script path as-passed (process.argv[1]).
  // Handles junction links: argv[1] may be the pre-resolved junction path,
  // while __dirname is the fully-resolved real path.
  const scriptDirs = [process.argv[1], __filename].filter(Boolean).map(path.dirname);
  for (const startDir of scriptDirs) {
    let dir = startDir;
    for (let i = 0; i < 6; i++) {
      const candidate = path.join(dir, "node_modules", pkgName, "bin", binaryName);
      if (fs.existsSync(candidate)) return candidate;
      const parent = path.dirname(dir);
      if (parent === dir) break; // reached filesystem root
      dir = parent;
    }
  }

  fatal(
    `Could not find the ${pkgName} package.\n` +
    `This usually means the optional dependency was skipped during install.\n` +
    `Try: npm install --include=optional\n` +
    `Or install directly: npm install ${pkgName}`
  );
}

// ── Execute ──────────────────────────────────────────────────────────────────

function main() {
  const binary = resolveBinary();

  // Make sure the binary is executable (npm may strip the bit on unpack).
  try {
    fs.chmodSync(binary, 0o755);
  } catch (_) {
    // Best-effort; Windows doesn't need it.
  }

  try {
    execFileSync(binary, process.argv.slice(2), { stdio: "inherit" });
  } catch (err) {
    // execFileSync throws when the child exits non-zero.
    // Mirror the child's exit code so shell pipelines work correctly.
    process.exit(err.status ?? 1);
  }
}

// ── Helpers ──────────────────────────────────────────────────────────────────

function fatal(msg) {
  process.stderr.write(`\n[forge] ERROR: ${msg}\n\n`);
  process.exit(1);
}

main();
