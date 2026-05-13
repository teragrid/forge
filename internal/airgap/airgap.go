// Copyright 2024 The Forge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package airgap implements M3-13: detection and configuration of air-gapped
// (offline) operation for forge.
//
// # Air-gap modes
//
//   - EnvForced   — FORGE_AIRGAP=1 is set explicitly
//   - AutoDetect  — probe fails; implies offline operation
//   - Online      — normal network access confirmed
//
// In air-gapped mode:
//   - Telemetry is suppressed automatically.
//   - LLM calls are routed to a local Ollama instance (OLLAMA_HOST) or
//     skipped with a clear error.
//   - Plugin registry downloads are blocked; plugins must be installed from
//     a local bundle (see Bundle* functions below).
//
// # Air-gap bundle
//
// A forge bundle is a directory (or tarball) with the following layout:
//
//	bundle/
//	  manifest.json        — bundle metadata + SHA-256 checksums
//	  forge-<os>-<arch>    — forge binary
//	  plugins/             — pre-built .wasm plugin files
//
// Create a bundle with `forge bundle create --out bundle.tar.gz`.
// Install from a bundle with `forge bundle extract --in bundle.tar.gz`.
package airgap

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/teragrid/forge/internal/errcode"
)

// Error codes (range 5900..5999).
var (
	ErrAirgapBundle = errcode.Register(errcode.Code(5900), "air-gap bundle operation failed")
	ErrAirgapProbe  = errcode.Register(errcode.Code(5901), "air-gap network probe failed")
)

// Mode describes the current air-gap state.
type Mode int

const (
	// ModeOnline indicates normal network access.
	ModeOnline Mode = iota
	// ModeForced indicates FORGE_AIRGAP=1 was set explicitly.
	ModeForced
	// ModeAutoDetected indicates offline status was detected via probe failure.
	ModeAutoDetected
)

func (m Mode) String() string {
	switch m {
	case ModeOnline:
		return "online"
	case ModeForced:
		return "airgap:forced"
	case ModeAutoDetected:
		return "airgap:auto-detected"
	default:
		return "unknown"
	}
}

// IsAirgapped returns true when the process is running in air-gap mode
// (either forced or auto-detected).
func (m Mode) IsAirgapped() bool { return m != ModeOnline }

// probeURL is used for the optional connectivity probe.
const probeURL = "https://registry.forge.dev/healthz"

// Detect returns the current air-gap Mode.
//
// If FORGE_AIRGAP=1 is set, returns ModeForced without making network calls.
// Otherwise, it attempts a quick HTTP probe (500 ms timeout). If the probe
// fails, it returns ModeAutoDetected.
func Detect(ctx context.Context) Mode {
	if v := os.Getenv("FORGE_AIRGAP"); v == "1" || v == "true" {
		return ModeForced
	}
	// Best-effort connectivity probe.
	client := &http.Client{
		Timeout: 500 * time.Millisecond,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: 300 * time.Millisecond}).DialContext,
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, probeURL, nil)
	if err != nil {
		return ModeAutoDetected
	}
	resp, err := client.Do(req)
	if err != nil {
		return ModeAutoDetected
	}
	defer resp.Body.Close() //nolint:errcheck
	return ModeOnline
}

// ── Bundle ────────────────────────────────────────────────────────────────────

// BundleManifest describes the contents of a forge offline bundle.
type BundleManifest struct {
	ForgeVersion string                  `json:"forge_version"`
	CreatedAt    string                  `json:"created_at"`
	Platform     string                  `json:"platform"` // "linux/amd64", "darwin/arm64", etc.
	Files        map[string]FileChecksum `json:"files"`    // relative path → checksum info
}

// FileChecksum holds integrity data for one file in the bundle.
type FileChecksum struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// ReadBundleManifest parses the manifest.json from a bundle directory.
// The path should point to the directory that contains manifest.json.
func ReadBundleManifest(bundleDir string) (*BundleManifest, error) {
	data, err := os.ReadFile(bundleDir + "/manifest.json")
	if err != nil {
		return nil, errcode.Newf(ErrAirgapBundle, err, "read bundle manifest from %s", bundleDir)
	}
	var m BundleManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, errcode.Newf(ErrAirgapBundle, err, "parse bundle manifest")
	}
	return &m, nil
}

// ValidateBundle checks that every file listed in the manifest exists.
// Full checksum verification is left to the caller (to avoid pulling in
// crypto/sha256 in the core detection path).
func ValidateBundle(bundleDir string) error {
	m, err := ReadBundleManifest(bundleDir)
	if err != nil {
		return err
	}
	var missing []string
	for rel := range m.Files {
		path := bundleDir + "/" + rel
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing = append(missing, rel)
		}
	}
	if len(missing) > 0 {
		return errcode.Newf(ErrAirgapBundle, nil,
			"bundle missing %d file(s): %v", len(missing), missing)
	}
	return nil
}

// Status returns a human-readable summary of air-gap status suitable for
// `forge doctor` output.
func Status(ctx context.Context) string {
	mode := Detect(ctx)
	switch mode {
	case ModeOnline:
		return fmt.Sprintf("network: online (mode=%s)", mode)
	case ModeForced:
		return fmt.Sprintf("network: OFFLINE — FORGE_AIRGAP=1 (mode=%s)", mode)
	default:
		return fmt.Sprintf("network: OFFLINE — probe failed (mode=%s)", mode)
	}
}
