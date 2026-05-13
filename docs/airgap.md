# Air-gapped Installation

> M3-13 — Offline / air-gapped install support for Forge

---

## Overview

Forge supports running in environments with no internet access — air-gapped
enterprise networks, offline developer machines, and secure facilities.

There are two air-gap scenarios:

1. **Install-time air-gap** — install the `forge` binary without downloading
   from the internet (pre-bundled artifacts).
2. **Run-time air-gap** — run `forge` commands without making outbound network
   calls (offline LLM via Ollama, no telemetry, no registry access).

---

## Install-time: offline bundle

A **forge bundle** is a directory (or `.tar.gz` tarball) containing:

```
bundle/
  manifest.json          ← bundle metadata + SHA-256 checksums
  forge-linux-amd64      ← forge binary for Linux/amd64
  forge-darwin-arm64     ← forge binary for macOS/Apple Silicon
  forge-windows-amd64.exe
  plugins/               ← pre-built .wasm plugins
    forge-plugin-gosec-v1.2.0.wasm
    forge-plugin-trivy-v0.1.0.wasm
```

### Creating a bundle (on an internet-connected machine)

```bash
forge bundle create \
  --version v1.0.0 \
  --platforms linux/amd64,darwin/arm64,windows/amd64 \
  --plugins gosec,trivy \
  --out forge-bundle-v1.0.0.tar.gz
```

### Transferring the bundle

Copy the tarball to the air-gapped machine via USB, corporate file share, or
other approved transfer mechanism.

### Extracting and installing (on the air-gapped machine)

```bash
# Extract
forge bundle extract --in forge-bundle-v1.0.0.tar.gz --out /opt/forge-bundle

# Validate checksums
forge bundle verify --dir /opt/forge-bundle

# Install the binary
sudo install -m 755 /opt/forge-bundle/forge-linux-amd64 /usr/local/bin/forge

# Install plugins from bundle
forge plugin install --from-bundle /opt/forge-bundle
```

---

## Run-time: offline mode

### Forcing air-gap mode

```bash
export FORGE_AIRGAP=1
forge doctor  # confirms: network: OFFLINE — FORGE_AIRGAP=1
```

### Auto-detection

When `FORGE_AIRGAP` is not set, forge probes `https://registry.forge.dev/healthz`
(500 ms timeout). If the probe fails, forge automatically switches to air-gap mode.

### Effects of air-gap mode

| Feature | Air-gap behaviour |
|---------|-------------------|
| Telemetry | Suppressed (no outbound calls) |
| Plugin registry | Blocked; use `--from-bundle` |
| LLM providers | Cloud providers are skipped; Ollama used if available |
| Update checks | Suppressed |
| `forge learn` | Queued locally (flushed when back online) |

---

## Local LLM with Ollama

[Ollama](https://ollama.ai) runs open-weight models locally with no internet.

### Setup

```bash
# Install Ollama (on the air-gapped machine, from a bundle)
# See: https://ollama.ai/download (download offline)

# Pull a model (do this before going offline)
ollama pull llama3.2
ollama pull codellama

# Point forge at the local Ollama server
export OLLAMA_HOST=http://localhost:11434
export OLLAMA_MODEL=codellama   # optional; default: llama3.2

forge doctor   # should show: LLM provider: ollama
```

### Supported models

Forge's Ollama adapter recognises any model served by Ollama, with recommended
defaults:

| Use case | Recommended model |
|----------|------------------|
| Code review / scan | `codellama` |
| General tasks | `llama3.2` |
| Small machines (<8 GB RAM) | `phi3` |

---

## `forge doctor` output in air-gap mode

```
forge doctor
  ✓ forge version: v1.0.0
  ✓ go: 1.24.0
  ✗ network: OFFLINE — FORGE_AIRGAP=1 (mode=airgap:forced)
  ✓ LLM provider: ollama (host=http://localhost:11434 model=codellama)
  ✓ telemetry: suppressed (air-gap)
  ✓ plugins: 2 installed (from bundle)
```

---

## CI/CD in air-gapped pipelines

In air-gapped CI environments, set `FORGE_AIRGAP=1` and `OLLAMA_HOST` in the
CI configuration. Use `forge bundle extract` as a pre-step to install plugins.

Example GitHub Actions step (self-hosted runner, no internet):

```yaml
- name: Install forge plugins (air-gap)
  env:
    FORGE_AIRGAP: "1"
    OLLAMA_HOST: "http://localhost:11434"
  run: |
    forge bundle extract --in ${{ runner.tool_cache }}/forge-bundle.tar.gz --out /tmp/forge-bundle
    forge plugin install --from-bundle /tmp/forge-bundle
    forge doctor
```

---

## Security considerations

- Always verify bundle checksums with `forge bundle verify` before use.
- Bundle manifests are signed with the Forge release key (Ed25519).
  Verify the signature with `forge bundle verify --sig bundle.tar.gz.sig`.
- Ollama models are not audited by Forge — use only models from trusted sources.
