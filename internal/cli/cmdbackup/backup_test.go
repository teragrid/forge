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

package cmdbackup

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TC-BACKUP-01: dry-run produces expected stdout without writing files.
func TestNew_DryRun(t *testing.T) {
	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected dry-run indicator in output, got: %q", out)
	}
}

// TC-BACKUP-02: --apply creates the snapshot manifest on disk.
func TestNew_Apply(t *testing.T) {
	dir := t.TempDir()

	// Seed a migration file so collectItems picks something up.
	mig := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(mig, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mig, "001_init.sql"), []byte("-- init"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--apply", "--root", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snapshotDir := filepath.Join(dir, backupDir)
	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		t.Fatalf("backup dir not created: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one snapshot manifest file")
	}

	data, err := os.ReadFile(filepath.Join(snapshotDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("cannot read snapshot: %v", err)
	}
	var m BackupManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("invalid snapshot JSON: %v", err)
	}
	if m.Mode != "apply" {
		t.Errorf("want mode=apply, got %q", m.Mode)
	}
	if len(m.Entries) == 0 {
		t.Error("expected at least one entry in the snapshot")
	}
}

// TC-BACKUP-03: --json emits valid JSON with expected fields.
func TestNew_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--apply", "--root", dir, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var m BackupManifest
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, buf.String())
	}
	if m.ID == "" {
		t.Error("want non-empty ID")
	}
	if m.Timestamp == "" {
		t.Error("want non-empty Timestamp")
	}
}

// TC-BACKUP-04: `forge backup list` with no backups prints friendly message.
func TestList_Empty(t *testing.T) {
	dir := t.TempDir()
	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"list", "--root", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "No backups") {
		t.Errorf("expected 'No backups' message, got: %q", buf.String())
	}
}

// TC-BACKUP-05: `forge backup list` after --apply shows the created snapshot.
func TestList_AfterApply(t *testing.T) {
	dir := t.TempDir()

	// First create a backup.
	createCmd := New()
	createCmd.SetOut(&bytes.Buffer{})
	createCmd.SetArgs([]string{"--apply", "--root", dir})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	// Now list it.
	listCmd := New()
	var buf bytes.Buffer
	listCmd.SetOut(&buf)
	listCmd.SetArgs([]string{"list", "--root", dir})

	if err := listCmd.Execute(); err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if strings.Contains(buf.String(), "No backups") {
		t.Errorf("expected backup entries, got: %q", buf.String())
	}
}
