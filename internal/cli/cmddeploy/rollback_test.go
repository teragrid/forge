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

// Gap test G-110: rollback writes an audit record to deploy history.
package cmddeploy

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRollback_AuditLog (G-110) verifies that `forge rollback --allow-irreversible`
// appends a rollback record to .forge/deploy-history.json.
func TestRollback_AuditLog(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Pre-populate deploy history with two records so rollback can find
	// the previous tag automatically.
	history := []DeployRecord{
		{Timestamp: "2024-01-01T00:00:00Z", Adapter: "shell", Target: "make deploy", Tag: "v1.0.0", Status: "ok"},
		{Timestamp: "2024-01-02T00:00:00Z", Adapter: "shell", Target: "make deploy", Tag: "v1.1.0", Status: "ok"},
	}
	forgeDir := filepath.Join(root, ".forge")
	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.MarshalIndent(history, "", "  ")
	if err := os.WriteFile(filepath.Join(root, ".forge", "deploy-history.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := NewRollback()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--root", root, "--allow-irreversible"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("forge rollback: %v\noutput: %s", err, buf.String())
	}

	// Read back the history and verify a rollback entry was appended.
	updated, err := loadDeployHistory(root)
	if err != nil {
		t.Fatalf("loadDeployHistory: %v", err)
	}
	if len(updated) < 3 {
		t.Fatalf("expected at least 3 records after rollback, got %d", len(updated))
	}
	last := updated[len(updated)-1]
	if !strings.Contains(last.Note, "rollback") {
		t.Errorf("last deploy record Note should contain 'rollback', got %q", last.Note)
	}
	if last.Tag != "v1.0.0" {
		t.Errorf("rollback should target previous tag v1.0.0, got %q", last.Tag)
	}
}

// TestRollback_DryRunNoAuditLog (G-110) verifies that --dry-run does NOT
// write to deploy history.
func TestRollback_DryRunNoAuditLog(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	forgeDir := filepath.Join(root, ".forge")
	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := NewRollback()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--root", root, "--dry-run", "--to", "v0.9.0"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("forge rollback --dry-run: %v\noutput: %s", err, buf.String())
	}

	// History file should not be created.
	histPath := filepath.Join(root, ".forge", "deploy-history.json")
	if _, err := os.Stat(histPath); err == nil {
		t.Error("deploy-history.json should not be created on --dry-run")
	}
}
