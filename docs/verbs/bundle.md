# forge bundle

Create and extract offline (air-gapped) install bundles.

## Synopsis

```
forge bundle create  --version <tag> [--platforms <list>] [--plugins <list>] --out <file>
forge bundle extract --in <file> --out <dir>
forge bundle verify  --dir <dir> [--sig <file>]
```

## Description

`forge bundle` supports fully air-gapped installation and operation.
See [airgap.md](../airgap.md) for the full guide.

### create

Downloads the forge binary and any requested plugins, writes a `manifest.json`
with SHA-256 checksums, and packages everything into a `.tar.gz`.

### extract

Unpacks a bundle tarball into a local directory.

### verify

Checks that every file in `manifest.json` exists and matches its declared checksum.
Optionally verifies the Ed25519 bundle signature.

## Flags

### create

| Flag | Required | Description |
|------|----------|-------------|
| `--version` | yes | Forge version to bundle (e.g. `v1.0.0`) |
| `--platforms` | no | Comma-separated `os/arch` list (default: current platform) |
| `--plugins` | no | Comma-separated plugin names to include |
| `--out` | yes | Output tarball path |

### extract

| Flag | Required | Description |
|------|----------|-------------|
| `--in` | yes | Input tarball path |
| `--out` | yes | Output directory |

### verify

| Flag | Required | Description |
|------|----------|-------------|
| `--dir` | yes | Bundle directory |
| `--sig` | no | Ed25519 signature file to verify |

## Examples

```bash
# Create a multi-platform bundle
forge bundle create \
  --version v1.0.0 \
  --platforms linux/amd64,darwin/arm64 \
  --plugins gosec,trivy \
  --out forge-bundle-v1.0.0.tar.gz

# Extract on an air-gapped machine
forge bundle extract --in forge-bundle-v1.0.0.tar.gz --out /opt/forge-bundle

# Verify checksums
forge bundle verify --dir /opt/forge-bundle
```

## Error codes

| Code | Meaning |
|------|---------|
| `FORGE-5900` | Air-gap bundle operation failed |
| `FORGE-5901` | Air-gap network probe failed |

## See also

- [Air-gapped Installation](../airgap.md)
- `forge doctor` — shows current network mode
