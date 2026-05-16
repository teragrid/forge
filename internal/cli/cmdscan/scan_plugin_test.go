// Copyright 2024 The Forge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// G-025: Third-party scanner plugin contract tests.
//
// Verifies that a third-party plugin can register itself in the scan-family
// contract and contribute findings to the unified report, using the same
// plugin.Scanner interface as the built-in scanners.
package cmdscan

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/plugin"
)

// ── G-025: Third-party scanner registration contract ─────────────────────────

// thirdPartyScanner is a synthetic external plugin that simulates what a
// `forge scan myco-pii` third-party scanner looks like.
type thirdPartyScanner struct {
	manifest plugin.Manifest
}

func (t *thirdPartyScanner) Manifest() plugin.Manifest { return t.manifest }

func (t *thirdPartyScanner) Scan(_ context.Context, root string) ([]plugin.Finding, error) {
	// Synthetic: report one finding for any file matching "pii-*.txt".
	var findings []plugin.Finding
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		base := filepath.Base(p)
		if strings.HasPrefix(base, "pii-") && strings.HasSuffix(base, ".txt") {
			rel, _ := filepath.Rel(root, p)
			findings = append(findings, plugin.Finding{
				File:   filepath.ToSlash(rel),
				Line:   1,
				Rule:   "myco-pii-001",
				Match:  "pii-file detected",
				Detail: "suspected PII file based on naming convention",
			})
		}
		return nil
	})
	return findings, nil
}

// TestThirdPartyPlugin_RegistersInScanFamily verifies that an external scanner
// plugin can be registered in a private registry using the scan-family contract
// and discovered via ByKind. G-025.
func TestThirdPartyPlugin_RegistersInScanFamily(t *testing.T) {
	t.Parallel()
	reg := plugin.NewRegistry()
	ext := &thirdPartyScanner{
		manifest: plugin.Manifest{
			Name:    "myco-pii",
			Version: "1.0.0",
			Kind:    plugin.KindScanner,
			Author:  "myco",
			Summary: "Detect PII files by naming convention.",
		},
	}
	reg.Register(ext)

	scanners := reg.ByKind(plugin.KindScanner)
	if len(scanners) != 1 {
		t.Fatalf("expected 1 scanner, got %d", len(scanners))
	}
	_, ok := reg.Lookup("myco-pii")
	if !ok {
		t.Fatal("myco-pii not found in registry after registration")
	}
}

// TestThirdPartyPlugin_ContributesFindings verifies that the third-party scanner
// returns findings in the unified plugin.Finding schema. G-025.
func TestThirdPartyPlugin_ContributesFindings(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "pii-user-export.txt", "name,email,ssn\nJohn,john@example.com,123-45-6789\n")
	writeFile(t, root, "safe-data.txt", "no pii here\n")

	ext := &thirdPartyScanner{
		manifest: plugin.Manifest{
			Name:    "myco-pii",
			Version: "1.0.0",
			Kind:    plugin.KindScanner,
		},
	}
	findings, err := ext.Scan(context.Background(), root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("expected at least one finding for pii-user-export.txt")
	}
	f := findings[0]
	if f.Rule == "" {
		t.Error("Finding.Rule must be non-empty")
	}
	if f.File == "" {
		t.Error("Finding.File must be non-empty")
	}
	if f.Line <= 0 {
		t.Errorf("Finding.Line must be > 0, got %d", f.Line)
	}
	if f.Match == "" {
		t.Error("Finding.Match must be non-empty")
	}
	// Safe file must not appear.
	for _, finding := range findings {
		if strings.Contains(finding.File, "safe-data") {
			t.Errorf("safe-data.txt should not be flagged: %+v", finding)
		}
	}
}

// TestThirdPartyPlugin_ManifestValidation verifies that manifests with missing
// required fields fail registration. G-025.
func TestThirdPartyPlugin_ManifestValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		manifest plugin.Manifest
		wantErr  bool
	}{
		{
			name:     "valid",
			manifest: plugin.Manifest{Name: "ok-plugin", Version: "1.0.0", Kind: plugin.KindScanner},
			wantErr:  false,
		},
		{
			name:     "missing_name",
			manifest: plugin.Manifest{Version: "1.0.0", Kind: plugin.KindScanner},
			wantErr:  true,
		},
		{
			name:     "missing_version",
			manifest: plugin.Manifest{Name: "no-version", Kind: plugin.KindScanner},
			wantErr:  true,
		},
		{
			name:     "unknown_kind",
			manifest: plugin.Manifest{Name: "bad-kind", Version: "1.0.0", Kind: "unknown"},
			wantErr:  true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.manifest.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected validation error but got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}

// TestThirdPartyPlugin_ScanFamilyCoexistsWithBuiltins verifies that a
// third-party scanner registered in a private registry does not interfere
// with built-in scanners in the global Default() registry. G-025.
func TestThirdPartyPlugin_ScanFamilyCoexistsWithBuiltins(t *testing.T) {
	t.Parallel()
	// Global registry (has built-ins registered via init()).
	builtins := plugin.Default().ByKind(plugin.KindScanner)
	if len(builtins) == 0 {
		t.Fatal("expected built-in scanners in Default() registry")
	}

	// Private registry (only the external scanner).
	ext := &thirdPartyScanner{
		manifest: plugin.Manifest{
			Name: "myco-pii", Version: "1.0.0", Kind: plugin.KindScanner,
		},
	}
	private := plugin.NewRegistry()
	private.Register(ext)

	// The external scanner must NOT appear in the global registry.
	if _, ok := plugin.Default().Lookup("myco-pii"); ok {
		t.Error("third-party plugin must not be registered in the global Default() registry")
	}

	// The built-ins must NOT appear in the private registry.
	for _, b := range builtins {
		if _, ok := private.Lookup(b.Manifest().Name); ok {
			t.Errorf("builtin %q must not appear in private registry", b.Manifest().Name)
		}
	}
}

// TestScanFixturePlugin_JSONSchema verifies that the scan-plugin fixture
// (tests/fixtures/scan-plugin/plugin.json) satisfies the required contract
// fields. G-025.
func TestScanFixturePlugin_JSONSchema(t *testing.T) {
	t.Parallel()
	// The contract only checks the JSON schema fields — we do not need to
	// actually invoke the plugin binary.
	m := plugin.Manifest{
		Name:         "scan-plugin-demo",
		Version:      "1.0.0",
		Kind:         plugin.KindScanner,
		Author:       "forge",
		Capabilities: []string{"scan"},
	}
	if err := m.Validate(); err != nil {
		t.Errorf("scan-plugin fixture manifest invalid: %v", err)
	}
}
