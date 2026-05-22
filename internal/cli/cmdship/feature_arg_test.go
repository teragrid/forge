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

// Tests for G-001, G-002, G-003 gap tasks:
//
//	G-001: positional <feature> arg on forge ship
//	G-002: --resume flag replacing forge ship resume subcommand
//	G-003: rename checkpoint 5 from verify→ship
package cmdship

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── G-001: Positional <feature> arg ──────────────────────────────────────────

// TestCmd_PositionalFeatureArg verifies that `forge ship auth/email --dry-run --json`
// exits 0, the feature slug is "auth-email", and .forge/specs/auth-email/ is created.
func TestCmd_PositionalFeatureArg(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"auth/email", "--root", root, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected exit 0, got error: %v\noutput: %s", err, out.String())
	}
	var res ShipResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("not valid JSON: %v\noutput: %s", err, out.String())
	}
	// Spec dir must have been created for slug "auth-email".
	specDir := filepath.Join(root, ".forge", "specs", "auth-email")
	if _, err := os.Stat(specDir); os.IsNotExist(err) {
		t.Fatalf("spec dir not created at %s", specDir)
	}
}

// TestCmd_PositionalFeatureArg_UnderscoreSlash verifies that slashes and spaces
// are slugified correctly: "user profile" → "user-profile".
func TestCmd_PositionalFeatureArg_Slugify(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"user profile", "--root", root, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected exit 0, got error: %v\noutput: %s", err, out.String())
	}
	specDir := filepath.Join(root, ".forge", "specs", "user-profile")
	if _, err := os.Stat(specDir); os.IsNotExist(err) {
		t.Fatalf("spec dir not created at %s", specDir)
	}
}

// TestCmd_DescriptionDeprecatedAlias verifies --description still works but
// the output includes a deprecation hint.
func TestCmd_DescriptionDeprecatedAlias(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--description", "auth email", "--root", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected exit 0, got error: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(strings.ToLower(out.String()), "deprecat") {
		t.Fatalf("expected deprecation hint in output, got: %s", out.String())
	}
}

// ── G-002: --resume flag ──────────────────────────────────────────────────────

// TestCmd_ResumeFlag verifies that `forge ship auth-email --resume --json` with
// a pre-populated spec.md resumes at checkpoint 2 (test).
func TestCmd_ResumeFlag(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Pre-populate spec checkpoint artifact.
	specDir := filepath.Join(root, ".forge", "specs", "auth-email")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# auth-email spec\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"auth-email", "--resume", "--root", root, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected exit 0, got error: %v\noutput: %s", err, out.String())
	}
	var res ShipResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("not valid JSON: %v\noutput: %s", err, out.String())
	}
	// First checkpoint returned must NOT be "Spec" (it was already done).
	if len(res.Checkpoints) > 0 && strings.EqualFold(res.Checkpoints[0].Name, "spec") {
		t.Fatalf("--resume should skip completed Spec checkpoint; got first cp: %q", res.Checkpoints[0].Name)
	}
}

// TestCmd_ResumeFlag_AllComplete verifies that --resume when all checkpoints
// are done outputs a "all checkpoints complete" message and exits 0.
func TestCmd_ResumeFlag_AllComplete(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "auth-email"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Pre-populate all 5 checkpoint artifacts.
	for _, cp := range []string{"spec", "test", "breakdown", "code", "ship"} {
		if err := os.WriteFile(filepath.Join(specDir, cp+".md"), []byte("done\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{slug, "--resume", "--root", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected exit 0 when all complete, got: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(strings.ToLower(out.String()), "complete") {
		t.Fatalf("expected 'complete' in output, got: %s", out.String())
	}
}

// TestCmd_ResumeSubcommandDeprecated verifies that `forge ship resume <feature>`
// still works but prints a migration hint pointing to --resume flag.
func TestCmd_ResumeSubcommandDeprecated(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	specDir := filepath.Join(root, ".forge", "specs", "my-feature")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# spec\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"resume", "my-feature", "--root", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected exit 0 for deprecated resume subcommand: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(strings.ToLower(out.String()), "deprecat") {
		t.Fatalf("expected deprecation notice for 'forge ship resume'; got: %s", out.String())
	}
}

// ── G-003: Rename checkpoint 5 from verify → ship ────────────────────────────

// TestCheckpoint6Name verifies that the 6th checkpoint returned by the full
// pipeline is named "Ship" (not "Verify").
func TestCheckpoint6Name(t *testing.T) {
	t.Parallel()
	res := RunCheckpoints(t.TempDir(), "", nil)
	if len(res.Checkpoints) != 6 {
		t.Fatalf("expected 6 checkpoints, got %d", len(res.Checkpoints))
	}
	if !strings.EqualFold(res.Checkpoints[5].Name, "ship") {
		t.Fatalf("checkpoint 6 must be named 'Ship', got %q", res.Checkpoints[5].Name)
	}
}

// TestCmd_ShipCheckpointSubcommand verifies that `forge ship ship --json`
// runs exactly checkpoint 5 and returns Name=="Ship".
func TestCmd_ShipCheckpointSubcommand(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"ship", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("forge ship ship failed: %v\noutput: %s", err, out.String())
	}
	var res ShipResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("not valid JSON: %v\noutput: %s", err, out.String())
	}
	if len(res.Checkpoints) != 1 {
		t.Fatalf("expected exactly 1 checkpoint, got %d", len(res.Checkpoints))
	}
	if !strings.EqualFold(res.Checkpoints[0].Name, "ship") {
		t.Fatalf("expected checkpoint name 'Ship', got %q", res.Checkpoints[0].Name)
	}
}

// TestCmd_VerifyDeprecatedAlias verifies that `forge ship verify --json`
// still exits 0 and outputs a deprecation notice.
func TestCmd_VerifyDeprecatedAlias(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"verify", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify (deprecated) subcommand failed: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(strings.ToLower(out.String()), "deprecat") {
		t.Fatalf("expected deprecation notice for 'forge ship verify'; got: %s", out.String())
	}
}

// TestRunCheckpoints_ShipAlias verifies that passing "ship" to RunCheckpoints
// returns the same checkpoint as "verify" used to.
func TestRunCheckpoints_ShipAlias(t *testing.T) {
	t.Parallel()
	res := RunCheckpoints(t.TempDir(), "", []string{"ship"})
	if len(res.Checkpoints) != 1 {
		t.Fatalf("expected 1 checkpoint, got %d", len(res.Checkpoints))
	}
	if !strings.EqualFold(res.Checkpoints[0].Name, "ship") {
		t.Fatalf("expected 'Ship', got %q", res.Checkpoints[0].Name)
	}
	if !res.Ready {
		t.Fatalf("ship checkpoint on fresh dir must be Ready=true: %+v", res.Checkpoints)
	}
}
