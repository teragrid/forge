# Authoring Forge Plugins

> This guide walks you from zero to a published Forge plugin.  You don't need to
> be a Go expert — the guide highlights exactly where your AI assistant can take
> over and what prompt to give it.

---

## What Is a Plugin?

A Forge plugin is a self-contained extension packaged as a
**WebAssembly (WASM)** binary.  Think of it like a browser extension: it adds
a new capability to Forge, runs in an isolated sandbox so it cannot harm your
machine, and can be installed by anyone from the Forge Registry with a single
command.

Out of the box Forge knows how to scan Go source code and scaffold a Go service.
Plugins let the community extend Forge to any language, framework, or workflow:

- A **scanner** plugin that detects hardcoded database credentials in Python
  files.
- A **codemod** plugin that upgrades `react` from v17 to v18 automatically.
- A **template** plugin that scaffolds a company-standard microservice.

---

## Plugin Kinds

| Kind | Interface | Purpose |
|------|-----------|---------|
| `scanner` | `Scanner` | Read files, return a list of findings |
| `codemod` | `Codemod` | Read files, return a list of file patches |
| `template` | `Template` | Receive variables, write a directory of files |
| `provider` | `Provider` | Wrap an external LLM or tool API |

---

## Prerequisites

| Tool | Purpose |
|------|---------|
| **Go 1.24 + TinyGo 0.31+** | Compile Go to WASM |
| **Forge CLI** | Scaffold, test, and publish |

Install TinyGo by following the [TinyGo installation guide](https://tinygo.org/getting-started/install/).

---

## 1. Scaffold the Plugin

Forge creates a ready-to-compile skeleton with one command:

```bash
forge new plugin my-scanner --module github.com/yourname/my-scanner
cd my-scanner
```

**What was just created:**

```
my-scanner/
├── plugin.toml        # Manifest: name, version, kind, capabilities
├── main.go            # Your plugin logic (start here)
├── main_test.go       # Unit test skeleton
├── .forge/
│   └── eval/
│       └── scenarios/ # YAML evaluation scenarios
└── Makefile           # build + test + pack targets
```

---

## 2. Describe the Plugin (`plugin.toml`)

`plugin.toml` is the ID card for your plugin.  Every field is validated by
Forge before the plugin is installed or run.

```toml
name    = "my-scanner"
version = "0.1.0"
kind    = "scanner"
author  = "Your Name <you@example.com>"
summary = "Detects hardcoded credentials in Python source files."

# Forge version constraint this plugin requires
forge_version = ">=0.2.0"

# Declare every permission the plugin needs.
# Users see these as an explicit consent prompt at install time.
# If a capability is not declared, the WASM host will deny the syscall at runtime.
capabilities = [
  "fs:read",   # Read files inside the project directory
  # "net:http" # Uncomment only if your plugin calls an external API
]
```

**Available capabilities:**

| Capability | What it grants |
|-----------|----------------|
| `fs:read` | Read files inside the project root |
| `fs:write` | Write files inside the project root (codemods need this) |
| `net:http` | Make outbound HTTP(S) requests |
| `env:read` | Read specific environment variables (must list them explicitly) |

> **Security note:** The WASM sandbox enforces these at the host level.  Even if
> malicious code inside your plugin calls `os.WriteFile`, the host will deny the
> syscall if `fs:write` is not declared.

---

## 3. Implement the Logic (`main.go`)

The scaffold generates a `main.go` that compiles to WASM and exports the entry
point Forge expects.  The key function to fill in is `Scan` (for a scanner):

```go
// Scan receives a request from the Forge host and returns findings.
// The host serialises req/resp as JSON across the WASM boundary.
func Scan(req plugin.ScanRequest) plugin.ScanResponse {
    var findings []plugin.Finding

    for _, file := range req.Files {
        if !strings.HasSuffix(file.Path, ".py") {
            continue
        }
        // Look for hard-coded passwords — your real logic goes here
        if bytes.Contains(file.Content, []byte("password=")) {
            findings = append(findings, plugin.Finding{
                Rule:    "hardcoded-password",
                File:    file.Path,
                Line:    detectLine(file.Content, "password="),
                Message: "Hard-coded password detected. Use an environment variable instead.",
            })
        }
    }

    return plugin.ScanResponse{Findings: findings}
}
```

> **Vibe-Coding Tip:** Paste the contents of
> [`internal/plugin/plugin.go`](../internal/plugin/plugin.go) into your AI
> prompt and say: *"I want a Forge scanner plugin that detects hardcoded
> passwords in Python files.  Use the ScanRequest and ScanResponse types shown
> here.  Write the complete Scan function."*

---

## 4. Write Evaluation Scenarios

Instead of hand-writing unit tests for complex edge cases, Forge lets you
describe them as YAML.  Create scenario files in `.forge/eval/scenarios/`:

```yaml
# .forge/eval/scenarios/happy-path.yaml
name: detects-hardcoded-password
plugin: my-scanner
kind: scanner
input:
  files:
    - path: app/config.py
      content: |
        database_url = "postgresql://localhost/mydb"
        password = "super_secret_123"
expected:
  findings:
    - rule: hardcoded-password
      file: app/config.py
```

```yaml
# .forge/eval/scenarios/no-false-positive.yaml
name: clean-file-produces-no-findings
plugin: my-scanner
kind: scanner
input:
  files:
    - path: app/config.py
      content: |
        import os
        password = os.environ["DB_PASSWORD"]
expected:
  findings: []   # must be empty — guards against over-eager matching
```

Run all scenarios:

```bash
forge eval .forge/eval/scenarios/
```

Green output means every scenario matched its expected result.  Your AI can
generate dozens of edge-case scenarios quickly — just ask it: *"Write 5 more
YAML eval scenarios for this scanner covering empty files, binary files, and
files with the word 'password' in a comment."*

---

## 5. Compile to WASM

```bash
# Using the generated Makefile
make build

# Manually with TinyGo
tinygo build -target=wasi -o my-scanner.wasm .
```

The output `my-scanner.wasm` is the distributable artefact.

---

## 6. Test Locally

Point Forge at your local WASM file to test before publishing:

```bash
# Add to .forge/plugins.json in your test project:
# { "plugins": [{ "path": "/absolute/path/to/my-scanner.wasm" }] }

forge plugin list            # should show my-scanner
forge scan all               # my-scanner will run alongside built-ins
```

---

## 7. Pack and Publish

Bundle your manifest and WASM binary together:

```bash
make pack
# Creates: my-scanner-0.1.0.tar.gz
```

Publish to the Forge Registry (alpha):

```bash
forge plugin publish ./my-scanner-0.1.0.tar.gz
```

After publishing, anyone can install your plugin with:

```bash
forge plugin install my-scanner
```

---

## 8. Compliance Checklist

Before submitting a plugin to the Registry, verify:

- [ ] All capabilities in `plugin.toml` are the minimum required.
- [ ] `forge eval` passes all scenarios including the false-positive guard.
- [ ] Plugin executes in < 500 ms on a 500-file project (NFR from `ARCHITECTURE.md §14`).
- [ ] No sensitive data is logged or returned in `ScanResponse.Debug` fields.
- [ ] `plugin.toml` has a valid semver `version` and a non-empty `summary`.
