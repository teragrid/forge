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

// migrate_test.go — migration runner tests (M2-22).
package cmdmigrate_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/cli/cmdmigrate"
)

// writeMigFile writes a minimal .up.sql migration file to dir.
func writeMigFile(t *testing.T, dir, version, name, sql string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	fname := filepath.Join(dir, version+"_"+name+".up.sql")
	if err := os.WriteFile(fname, []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
}

// makeRoot creates a temp project root with a migrations/ directory.
func makeRoot(t *testing.T) (root, migDir string) {
	t.Helper()
	root = t.TempDir()
	migDir = filepath.Join(root, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, migDir
}

// TestStatusEmpty verifies `migrate status` on a project with no migrations.
func TestStatusEmpty(t *testing.T) {
	root, _ := makeRoot(t)
	cmd := cmdmigrate.New()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"status", "--root", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate status: unexpected error: %v", err)
	}
}

// TestUpAppliesPending verifies that `migrate up` marks pending migrations as applied.
func TestUpAppliesPending(t *testing.T) {
	root, migDir := makeRoot(t)
	writeMigFile(t, migDir, "001", "create_users",
		"CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT);")
	writeMigFile(t, migDir, "002", "add_email",
		"ALTER TABLE users ADD COLUMN email TEXT;")

	cmd := cmdmigrate.New()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"up", "--root", root, "--dir", migDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate up: unexpected error: %v", err)
	}

	// Re-run status and confirm 2 applied, 0 pending.
	cmd2 := cmdmigrate.New()
	out := &bytes.Buffer{}
	cmd2.SetOut(out)
	cmd2.SetErr(out)
	cmd2.SetArgs([]string{"status", "--root", root, "--dir", migDir, "--json"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("migrate status: unexpected error: %v", err)
	}
	var res cmdmigrate.MigrateResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("json decode: %v\nraw: %s", err, out.String())
	}
	if len(res.Applied) != 2 {
		t.Errorf("Applied = %d; want 2", len(res.Applied))
	}
	if len(res.Pending) != 0 {
		t.Errorf("Pending = %d; want 0", len(res.Pending))
	}
}

// TestUpLimited verifies `migrate up 1` only applies the first pending migration.
func TestUpLimited(t *testing.T) {
	root, migDir := makeRoot(t)
	writeMigFile(t, migDir, "001", "first", "SELECT 1;")
	writeMigFile(t, migDir, "002", "second", "SELECT 2;")

	cmd := cmdmigrate.New()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"up", "1", "--root", root, "--dir", migDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate up 1: unexpected error: %v", err)
	}

	cmd2 := cmdmigrate.New()
	out := &bytes.Buffer{}
	cmd2.SetOut(out)
	cmd2.SetErr(out)
	cmd2.SetArgs([]string{"status", "--root", root, "--dir", migDir, "--json"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("migrate status: %v", err)
	}
	var res cmdmigrate.MigrateResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if len(res.Applied) != 1 {
		t.Errorf("Applied = %d; want 1", len(res.Applied))
	}
	if len(res.Pending) != 1 {
		t.Errorf("Pending = %d; want 1", len(res.Pending))
	}
}

// TestDryRunDoesNotPersist verifies that --dry-run does not update history.
func TestDryRunDoesNotPersist(t *testing.T) {
	root, migDir := makeRoot(t)
	writeMigFile(t, migDir, "001", "users", "SELECT 1;")

	cmd := cmdmigrate.New()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"up", "--root", root, "--dir", migDir, "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate up --dry-run: %v", err)
	}

	// History file must not exist after a dry run.
	histFile := filepath.Join(root, ".forge", "migrations", "history.json")
	if _, err := os.Stat(histFile); !os.IsNotExist(err) {
		t.Errorf("history file should not exist after dry-run, but it does")
	}
}

// TestDownRequiresFlag verifies that `migrate down` without --allow-irreversible fails.
func TestDownRequiresFlag(t *testing.T) {
	root, migDir := makeRoot(t)
	writeMigFile(t, migDir, "001", "users", "SELECT 1;")

	// First, apply one migration.
	up := cmdmigrate.New()
	up.SetOut(&bytes.Buffer{})
	up.SetErr(&bytes.Buffer{})
	up.SetArgs([]string{"up", "--root", root, "--dir", migDir})
	if err := up.Execute(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	// Down without flag must error.
	down := cmdmigrate.New()
	down.SetOut(&bytes.Buffer{})
	down.SetErr(&bytes.Buffer{})
	down.SetArgs([]string{"down", "--root", root, "--dir", migDir})
	if err := down.Execute(); err == nil {
		t.Error("migrate down without --allow-irreversible: expected error, got nil")
	}
}

// TestDownRollsBack verifies `migrate down --allow-irreversible` rolls back one migration.
func TestDownRollsBack(t *testing.T) {
	root, migDir := makeRoot(t)
	writeMigFile(t, migDir, "001", "users", "SELECT 1;")
	writeMigFile(t, migDir, "002", "posts", "SELECT 2;")

	// Apply both.
	up := cmdmigrate.New()
	up.SetOut(&bytes.Buffer{})
	up.SetErr(&bytes.Buffer{})
	up.SetArgs([]string{"up", "--root", root, "--dir", migDir})
	if err := up.Execute(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	// Roll back one.
	down := cmdmigrate.New()
	down.SetOut(&bytes.Buffer{})
	down.SetErr(&bytes.Buffer{})
	down.SetArgs([]string{"down", "--root", root, "--dir", migDir, "--allow-irreversible"})
	if err := down.Execute(); err != nil {
		t.Fatalf("migrate down: %v", err)
	}

	// Confirm 1 applied, 1 pending.
	status := cmdmigrate.New()
	out := &bytes.Buffer{}
	status.SetOut(out)
	status.SetErr(out)
	status.SetArgs([]string{"status", "--root", root, "--dir", migDir, "--json"})
	if err := status.Execute(); err != nil {
		t.Fatalf("migrate status: %v", err)
	}
	var res cmdmigrate.MigrateResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if len(res.Applied) != 1 {
		t.Errorf("Applied = %d; want 1", len(res.Applied))
	}
	if len(res.Pending) != 1 {
		t.Errorf("Pending = %d; want 1", len(res.Pending))
	}
}

// TestJSONOutput verifies that --json emits valid JSON.
func TestJSONOutput(t *testing.T) {
	root, migDir := makeRoot(t)
	writeMigFile(t, migDir, "001", "create_x", "SELECT 1;")

	cmd := cmdmigrate.New()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"status", "--root", root, "--dir", migDir, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate status --json: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(out.String()), "{") {
		t.Errorf("expected JSON output, got: %s", out.String())
	}
	var result cmdmigrate.MigrateResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Errorf("JSON parse error: %v", err)
	}
}
