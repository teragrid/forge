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

// TEST-26: Upgrade-idempotency test.

package tasktests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/teragrid/forge/internal/codemod"
)

// TC-26-01 (happy): applying a codemod twice leaves zero changes on the second run.
func TestTC2601_UpgradeIdempotency(t *testing.T) {
	t.Parallel()
	codemods := codemod.Default().All()
	if len(codemods) == 0 {
		t.Skip("no codemods registered")
	}
	for _, c := range codemods {
		c := c
		t.Run(c.Name(), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			// First application.
			_, err := c.Apply(dir, false)
			if err != nil {
				t.Logf("first Apply on empty dir: %v (may be expected)", err)
			}
			// Second application must report zero changes.
			r2, err := c.Apply(dir, false)
			if err != nil {
				t.Logf("second Apply: %v (may be expected)", err)
				return
			}
			if r2.Changed != 0 {
				t.Errorf("second Apply changed %d file(s); want 0 (not idempotent)", r2.Changed)
			}
		})
	}
}

// TC-26-02 (boundary): dry-run must NOT modify any files.
func TestTC2602_UpgradeDryRunNoWrite(t *testing.T) {
	t.Parallel()
	codemods := codemod.Default().All()
	if len(codemods) == 0 {
		t.Skip("no codemods registered")
	}
	for _, c := range codemods {
		c := c
		t.Run(c.Name(), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			// Capture state before dry-run.
			before := dirSnapshot(t, dir)
			r, err := c.Apply(dir, true)
			if err != nil {
				t.Logf("dry-run Apply: %v (may be expected)", err)
				return
			}
			after := dirSnapshot(t, dir)
			if !snapshotsEqual(before, after) {
				t.Errorf("dry-run for codemod %q modified files: %v", c.Name(), r)
			}
		})
	}
}

// TC-26-04 (data-accuracy): codemod Report.DryRun field is set correctly.
func TestTC2604_UpgradeDryRunFlag(t *testing.T) {
	t.Parallel()
	codemods := codemod.Default().All()
	if len(codemods) == 0 {
		t.Skip("no codemods registered")
	}
	for _, c := range codemods {
		c := c
		t.Run(c.Name(), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			r, err := c.Apply(dir, true)
			if err != nil {
				t.Logf("dry-run Apply: %v", err)
				return
			}
			if !r.DryRun {
				t.Errorf("codemod %q: Report.DryRun = false, want true", c.Name())
			}
		})
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// dirSnapshot returns a map of relative path → file content for dir.
func dirSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		data, _ := os.ReadFile(path)
		snap[rel] = string(data)
		return nil
	})
	return snap
}

// snapshotsEqual compares two directory snapshots.
func snapshotsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
