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

package cmdtemplates_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/cli/cmdtemplates"
)

// ── TMPL-INIT-01: happy path — writes .forge/tsd.yml ─────────────────────────

func TestTemplatesInit_HappyPath(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()

	cmd := cmdtemplates.New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"init", "--from", "promotiai", "--out", outDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, buf.String())
	}

	tsdPath := filepath.Join(outDir, ".forge", "tsd.yml")
	data, err := os.ReadFile(tsdPath)
	if err != nil {
		t.Fatalf("tsd.yml not created: %v", err)
	}
	if !strings.Contains(string(data), "tsd_version: 1") {
		t.Errorf("tsd.yml missing 'tsd_version: 1'; content:\n%s", data)
	}
	if !strings.Contains(string(data), "stripe") {
		t.Errorf("tsd.yml missing 'stripe' payment provider; content:\n%s", data)
	}
}

// ── TMPL-INIT-02: unknown template ID returns error ───────────────────────────

func TestTemplatesInit_UnknownID(t *testing.T) {
	t.Parallel()
	cmd := cmdtemplates.New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"init", "--from", "does-not-exist"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown template ID, got nil")
	}
	if !strings.Contains(err.Error(), "does-not-exist") && !strings.Contains(buf.String(), "does-not-exist") {
		t.Errorf("error message should reference the unknown ID; err=%v output=%s", err, buf.String())
	}
}

// ── TMPL-INIT-03: existing tsd.yml without --overwrite returns error ──────────

func TestTemplatesInit_ExistingNoOverwrite(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	forgeDir := filepath.Join(outDir, ".forge")
	if err := os.MkdirAll(forgeDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(forgeDir, "tsd.yml"), []byte("tsd_version: 1\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	cmd := cmdtemplates.New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"init", "--from", "promotiai", "--out", outDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when tsd.yml exists without --overwrite")
	}
}

// ── TMPL-INIT-04: --overwrite replaces existing tsd.yml ──────────────────────

func TestTemplatesInit_Overwrite(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	forgeDir := filepath.Join(outDir, ".forge")
	if err := os.MkdirAll(forgeDir, 0o750); err != nil {
		t.Fatal(err)
	}
	oldContent := "tsd_version: 1\n# old content\n"
	tsdPath := filepath.Join(forgeDir, "tsd.yml")
	if err := os.WriteFile(tsdPath, []byte(oldContent), 0o640); err != nil {
		t.Fatal(err)
	}

	cmd := cmdtemplates.New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"init", "--from", "promotiai", "--out", outDir, "--overwrite"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error with --overwrite: %v", err)
	}

	data, _ := os.ReadFile(tsdPath)
	if strings.Contains(string(data), "# old content") {
		t.Errorf("file was not overwritten; still contains old content")
	}
}

// ── TMPL-INIT-05: missing --from flag returns helpful error ───────────────────

func TestTemplatesInit_MissingFrom(t *testing.T) {
	t.Parallel()
	cmd := cmdtemplates.New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"init"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --from is missing")
	}
}

// ── TMPL-INIT-06: go-cloud-native template is also registered ────────────────

func TestTemplatesInit_GoCloudNative(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()

	cmd := cmdtemplates.New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"init", "--from", "go-cloud-native", "--out", outDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, buf.String())
	}

	tsdPath := filepath.Join(outDir, ".forge", "tsd.yml")
	if _, err := os.Stat(tsdPath); err != nil {
		t.Fatalf("tsd.yml not created for go-cloud-native: %v", err)
	}
}
