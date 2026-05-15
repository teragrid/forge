# @forgeone/cli

> **AI-native developer framework** — scaffold, lint, scan, and ship production-grade services without touching Go.

## Install

```sh
npm install -g @forgeone/cli
# or run without installing
npx @forgeone/cli new my-app --template ts-service
```

No Go installation required. The right pre-compiled binary is pulled automatically for your platform.

## Quick start

```sh
# Scaffold a new TypeScript/NestJS service
forge new my-app --template ts-service
cd my-app && npm install && npm run dev

# Or initialise the current directory (like git init / npm init)
mkdir my-app && cd my-app
forge init

# List all available templates
forge new --list
```

## Supported platforms

| Platform | Package |
|---|---|
| Linux x64 | `@forgeone/cli-linux-x64` |
| Linux arm64 | `@forgeone/cli-linux-arm64` |
| macOS x64 | `@forgeone/cli-darwin-x64` |
| macOS arm64 (Apple Silicon) | `@forgeone/cli-darwin-arm64` |
| Windows x64 | `@forgeone/cli-win32-x64` |

## Commands

| Command | What it does |
|---|---|
| `forge new <template> <name>` | Scaffold a new project in a new directory |
| `forge init [path]` | Initialise an existing directory as a Forge project |
| `forge lint` | Run Forge hygiene linting |
| `forge scan secrets` | Scan for leaked secrets |
| `forge scan security` | Run SAST security scan |
| `forge doctor` | Health-check the current project |
| `forge ship` | Production readiness gate |
| `forge eval` | Run AI eval scenarios |

Run `forge --help` for the full command reference.

## Building from source

```sh
git clone https://github.com/teragrid/forge
cd forge
go build -o dist/forge ./cmd/forge
```

Requires Go 1.24+.

## License

Apache-2.0 — see [LICENSE](https://github.com/teragrid/forge/blob/main/LICENSE).
