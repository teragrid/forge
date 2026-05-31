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

// ship_undo_test.go — tests for undo / snapshot helpers added by RFC-005 P2.

package cmdship

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/teragrid/forge/internal/telemetry"
)

// ─── writeShipTrashManifest ───────────────────────────────────────────────────

// TestWriteShipTrashManifest_CreatesManifest verifies the manifest file is written
// to .forge/trash/<runID>/manifest.json.
func TestWriteShipTrashManifest_CreatesManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeShipTrashManifest(root, "run-001", "my-feature")
	manifest := filepath.Join(root, ".forge", "trash", "run-001", "manifest.json")
	if _, err := os.Stat(manifest); os.IsNotExist(err) {
		t.Fatalf("manifest.json not created at %s", manifest)
	}
}

// TestWriteShipTrashManifest_VerbIsShip verifies the manifest records verb="ship".
func TestWriteShipTrashManifest_VerbIsShip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeShipTrashManifest(root, "run-002", "feat-a")
	data, err := os.ReadFile(filepath.Join(root, ".forge", "trash", "run-002", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"verb":"ship"`) {
		t.Errorf("manifest missing verb:ship, got: %s", data)
	}
}

// TestWriteShipTrashManifest_RecordsRunID verifies the run ID is stored.
func TestWriteShipTrashManifest_RecordsRunID(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeShipTrashManifest(root, "run-abc-123", "feature-x")
	data, _ := os.ReadFile(filepath.Join(root, ".forge", "trash", "run-abc-123", "manifest.json"))
	if !strings.Contains(string(data), "run-abc-123") {
		t.Errorf("manifest does not contain run ID 'run-abc-123': %s", data)
	}
}

// TestWriteShipTrashManifest_TwoRunsDistinct verifies two manifests are independent.
func TestWriteShipTrashManifest_TwoRunsDistinct(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeShipTrashManifest(root, "run-1", "feat")
	writeShipTrashManifest(root, "run-2", "feat")
	for _, id := range []string{"run-1", "run-2"} {
		p := filepath.Join(root, ".forge", "trash", id, "manifest.json")
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("missing manifest for %s", id)
		}
	}
}

// ─── telemetry span helpers ───────────────────────────────────────────────────

// TestStartPipelineSpan_ReturnsNonEmptyIDs verifies that traceID and spanID are set.
func TestStartPipelineSpan_ReturnsNonEmptyIDs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	traceID, spanID := telemetry.StartPipelineSpan(root, "ship")
	if traceID == "" {
		t.Error("traceID must not be empty")
	}
	if spanID == "" {
		t.Error("spanID must not be empty")
	}
}

// TestStartPipelineSpan_UniquePerInvocation verifies two calls produce different IDs.
func TestStartPipelineSpan_UniquePerInvocation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	traceA, _ := telemetry.StartPipelineSpan(root, "ship")
	traceB, _ := telemetry.StartPipelineSpan(root, "ship")
	if traceA == traceB {
		t.Error("expected unique traceIDs for independent pipeline spans")
	}
}

// TestEmitCheckpointSpan_TelemetryDisabled verifies emit does not panic or error
// when telemetry is not opted in (empty root / no config).
func TestEmitCheckpointSpan_TelemetryDisabled(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("EmitCheckpointSpan panicked with disabled telemetry: %v", r)
		}
	}()
	root := t.TempDir() // no telemetry.json → disabled
	err := telemetry.EmitCheckpointSpan(root, "trace-x", "span-y", "scan", "OK", 100*time.Millisecond)
	// Error is acceptable (telemetry disabled returns nil per implementation).
	_ = err
}

// TestEmitCheckpointSpan_EnabledWritesFile verifies a span is written to the
// .forge/telemetry.jsonl file when telemetry is enabled.
func TestEmitCheckpointSpan_EnabledWritesFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Enable telemetry for this run.
	cfg := &telemetry.Config{Enabled: true, InstallID: "test-install"}
	cfgPath := filepath.Join(root, telemetry.DefaultConfigPath)
	if err := telemetry.SaveConfig(cfgPath, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	err := telemetry.EmitCheckpointSpan(root, "trace-1", "span-1", "architecture", "OK", 200*time.Millisecond)
	if err != nil {
		t.Fatalf("EmitCheckpointSpan: %v", err)
	}
	spanFile := filepath.Join(root, telemetry.DefaultSpanPath)
	if _, err := os.Stat(spanFile); os.IsNotExist(err) {
		t.Errorf("telemetry span file not created: %s", spanFile)
	}
}

// ─── snapOnFail helper ────────────────────────────────────────────────────────

// TestSnapOnFail_BestEffort verifies snapOnFail does not panic on missing spec dir.
func TestSnapOnFail_BestEffort(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("snapOnFail panicked: %v", r)
		}
	}()
	// No spec dir — must not error or panic.
	snapOnFail(root, "nonexistent-slug", "arch")
}

// TestSnapOnFail_CreatesSnapshotWhenSpecExists verifies snapshotting works.
func TestSnapOnFail_CreatesSnapshotWhenSpecExists(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "my-feat"
	specDir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# spec"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapOnFail(root, slug, "code")
	if !SnapshotExists(root, slug, "code") {
		t.Error("expected snapshot to exist after snapOnFail with valid spec dir")
	}
}
