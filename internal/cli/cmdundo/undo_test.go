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
package cmdundo

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestListSnapshotManifests_NoTrashDir verifies that listSnapshotManifests
// returns nil, nil when the .forge/trash directory does not exist (ADR-024).
func TestListSnapshotManifests_NoTrashDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// .forge/trash does not exist.
	manifests, err := listSnapshotManifests(root)
	if err != nil {
		t.Fatalf("listSnapshotManifests must not error when trash dir absent: %v", err)
	}
	if len(manifests) != 0 {
		t.Errorf("expected empty slice when trash dir absent, got %d manifests", len(manifests))
	}
}

// TestNew_ListEmpty verifies that --list on an empty root prints an informational
// message and exits 0.
func TestNew_ListEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--root", root, "--list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--list on empty trash must not error: %v", err)
	}
	if !strings.Contains(out.String(), "no snapshots") {
		t.Errorf("expected 'no snapshots' in output, got: %s", out.String())
	}
}

// TestNew_ListJSON_Empty verifies that --list --json returns valid JSON (null
// or empty array) when no snapshots exist.
func TestNew_ListJSON_Empty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--root", root, "--list", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--list --json on empty trash must not error: %v", err)
	}
	// JSON must be valid; null represents an empty snapshot list.
	body := bytes.TrimSpace(out.Bytes())
	var snaps []TrashManifest
	if err := json.Unmarshal(body, &snaps); err != nil {
		t.Fatalf("--list --json must produce valid JSON: %v\noutput: %s", err, out.String())
	}
}

// TestNew_UndoEmpty verifies that forge undo with no snapshots prints an
// informational message and exits 0 (ADR-024).
func TestNew_UndoEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--root", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("undo with no snapshots must not error: %v", err)
	}
	if !strings.Contains(out.String(), "no snapshots") {
		t.Errorf("expected 'no snapshots' in output, got: %s", out.String())
	}
}
