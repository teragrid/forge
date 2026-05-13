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
package cmdscan

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestScan_SecretsClean(t *testing.T) {
	t.Parallel()
	// Smoke test that scan runs without crashing on a clean project.
	res, err := RunSecrets(t.TempDir())
	if err != nil {
		t.Fatalf("RunSecrets: %v", err)
	}
	if res.Status != "clean" && len(res.Findings) == 0 {
		t.Fatalf("empty dir should be clean, got status=%q findings=%d", res.Status, len(res.Findings))
	}
}

func TestCmd_Text(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"secrets", "--root", t.TempDir()})
	_ = cmd.Execute() // may fail on non-clean, but should not crash
	if !strings.Contains(out.String(), "forge scan") {
		t.Fatalf("missing header: %s", out.String())
	}
}

func TestCmd_JSON(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"secrets", "--root", t.TempDir(), "--json"})
	_ = cmd.Execute()

	var res ScanResult
	body := bytes.TrimSpace(out.Bytes())
	if i := bytes.LastIndexByte(body, '}'); i >= 0 {
		body = body[:i+1]
	}
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if res.Status == "" {
		t.Fatal("expected Status in JSON output")
	}
}

func TestCmd_UnknownScanner(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"no-such-scanner"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown scanner")
	}
	if !strings.Contains(err.Error(), "FORGE-3000") {
		t.Fatalf("want FORGE-3000, got: %v", err)
	}
}

// TestScan_SecurityFamilyRunsAllSubScanners verifies the spec §4 "security"
// family delegates to all M1 security sub-scanners and returns a merged result.
func TestScan_SecurityFamilyRunsAllSubScanners(t *testing.T) {
	t.Parallel()
	res, err := RunSecurity(t.TempDir())
	if err != nil {
		t.Fatalf("RunSecurity: %v", err)
	}
	if res.Status == "" {
		t.Fatal("RunSecurity: empty Status")
	}
	// Clean dir should be clean
	if res.Status != "clean" {
		t.Errorf("clean tempdir should be clean, got %q (count=%d)", res.Status, res.Count)
	}
}

// TestScan_SpecFamilyNamesAccepted verifies all spec §4 family names are accepted
// (no "unknown scanner" error). This is the primary backward-compat guard.
func TestScan_SpecFamilyNamesAccepted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specFamilies := []string{
		"security", "correctness", "performance", "reliability",
		"accessibility", "cost", "compliance", "dx", "all",
	}
	for _, family := range specFamilies {
		family := family
		t.Run(family, func(t *testing.T) {
			t.Parallel()
			cmd := New()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{family, "--root", dir})
			if err := cmd.Execute(); err != nil {
				// Findings cause non-zero exit but are not "unknown scanner" errors.
				if strings.Contains(err.Error(), "unknown scanner") {
					t.Errorf("family %q should be accepted, got 'unknown scanner': %v", family, err)
				}
			}
		})
	}
}

// TestScan_LegacyAliasesStillWork verifies the M1 sub-family aliases remain
// backward-compatible (false-positive guard: renaming spec families must not
// break existing --scan secrets|rls|prompt-injection|supply-chain usage).
func TestScan_LegacyAliasesStillWork(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	legacyNames := []string{"secrets", "rls", "prompt-injection", "supply-chain"}
	for _, name := range legacyNames {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cmd := New()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{name, "--root", dir})
			if err := cmd.Execute(); err != nil {
				if strings.Contains(err.Error(), "unknown scanner") {
					t.Errorf("legacy alias %q must still work, got 'unknown scanner': %v", name, err)
				}
			}
		})
	}
}

// TestScan_M2FamiliesRunClean verifies all M2 families run without error on an empty directory.
func TestScan_M2FamiliesRunClean(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	families := map[string]func(string) (*ScanResult, error){
		"correctness":   RunCorrectness,
		"performance":   RunPerformance,
		"reliability":   RunReliability,
		"accessibility": RunAccessibility,
		"cost":          RunCost,
		"compliance":    RunCompliance,
		"dx":            RunDX,
	}
	for name, run := range families {
		name, run := name, run
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			res, err := run(dir)
			if err != nil {
				t.Fatalf("Run%s: unexpected error: %v", name, err)
			}
			if res == nil {
				t.Fatalf("Run%s: nil result", name)
			}
			// An empty directory may produce "missing-forge-manifest" from DX; that's fine.
			// All other families must return no findings on an empty directory.
			if name != "dx" && res.Count != 0 {
				t.Errorf("Run%s: empty dir should yield 0 findings, got %d: %+v", name, res.Count, res.Findings)
			}
		})
	}
}
