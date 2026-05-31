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

// snapshot.go — P2: Transactional checkpoint snapshots.
//
// Before each checkpoint the pipeline may call TakeSnapshot to copy all
// artefact files in .forge/specs/<slug>/ to
// .forge/.snapshots/<slug>/<checkpoint>/.
//
// On checkpoint failure the caller can invoke RestoreSnapshot to revert to the
// pre-checkpoint state.  This provides all-or-nothing semantics for the spec
// artefacts directory, which is critical for idempotent replay and forge undo.
//
// Design decisions:
//   - File-level copy only (no git operations) so it works on detached HEAD
//     and in CI environments where git operations may be restricted.
//   - Top-level files only; nested subdirectories (e.g. digests/) are
//     intentionally excluded — they are rebuilt deterministically on replay.
//   - A meta.txt file records checkpoint + timestamp for audit purposes.
//   - All operations are best-effort on TakeSnapshot; RestoreSnapshot returns
//     an error when the snapshot cannot be found.
package cmdship

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const snapshotsBaseDir = ".snapshots"

// TakeSnapshot copies the top-level files from .forge/specs/<slug>/ to
// .forge/.snapshots/<slug>/<checkpoint>/ and writes a meta.txt file.
// Safe to call when the specs directory does not yet exist (no-op).
// Returns an error only when the snapshot destination cannot be created.
func TakeSnapshot(root, slug, checkpoint string) error {
	src := filepath.Join(root, ".forge", "specs", slug)
	dst := filepath.Join(root, ".forge", snapshotsBaseDir, slug, checkpoint)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("snapshot mkdir %s: %w", dst, err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		// Source does not exist — valid for the very first checkpoint.
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue // skip nested dirs (digests/, etc.)
		}
		if err := snapshotCopyFile(
			filepath.Join(src, e.Name()),
			filepath.Join(dst, e.Name()),
		); err != nil {
			return fmt.Errorf("snapshot copy %s: %w", e.Name(), err)
		}
	}

	meta := fmt.Sprintf("checkpoint: %s\ntaken_at: %s\n",
		checkpoint, time.Now().UTC().Format(time.RFC3339))
	_ = os.WriteFile(filepath.Join(dst, "meta.txt"), []byte(meta), 0o600)
	return nil
}

// RestoreSnapshot copies files from .forge/.snapshots/<slug>/<checkpoint>/ back
// to .forge/specs/<slug>/.  Only files present in the snapshot are restored;
// files not in the snapshot are left untouched (conservative restore).
// Returns an error when no snapshot exists for the given slug+checkpoint.
func RestoreSnapshot(root, slug, checkpoint string) error {
	src := filepath.Join(root, ".forge", snapshotsBaseDir, slug, checkpoint)
	dst := filepath.Join(root, ".forge", "specs", slug)

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("restore: no snapshot at %s: %w", src, err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("restore mkdir %s: %w", dst, err)
	}
	for _, e := range entries {
		if e.IsDir() || e.Name() == "meta.txt" {
			continue
		}
		if err := snapshotCopyFile(
			filepath.Join(src, e.Name()),
			filepath.Join(dst, e.Name()),
		); err != nil {
			return fmt.Errorf("restore copy %s: %w", e.Name(), err)
		}
	}
	return nil
}

// SnapshotExists reports whether a snapshot has been taken for slug+checkpoint.
func SnapshotExists(root, slug, checkpoint string) bool {
	_, err := os.Stat(
		filepath.Join(root, ".forge", snapshotsBaseDir, slug, checkpoint, "meta.txt"))
	return err == nil
}

// ListSnapshots returns the checkpoint names for which snapshots exist for slug.
// Returns nil when no snapshot directory is present.
func ListSnapshots(root, slug string) []string {
	dir := filepath.Join(root, ".forge", snapshotsBaseDir, slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

// snapshotCopyFile copies a single regular file from src to dst.
func snapshotCopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// writeShipTrashManifest writes a TrashManifest for a `forge ship` run so that
// `forge undo` can locate and reverse it.
//
// The manifest is written to .forge/trash/<runID>/manifest.json.
// This satisfies ADR-024 (§17.1 #5 reversibility contract) for the ship verb.
// Errors are silently swallowed — a missing manifest is recoverable by the user.
func writeShipTrashManifest(root, runID, specSlug string) {
	dir := filepath.Join(root, ".forge", "trash", runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}

	// Record the spec artefacts directory as the primary tracked path.
	specDir := filepath.Join(root, ".forge", "specs", specSlug)
	type trashFile struct {
		OrigPath string `json:"orig_path"`
		SavePath string `json:"save_path"`
		Mode     uint32 `json:"mode"`
	}
	type trashManifest struct {
		RunID     string      `json:"run_id"`
		Timestamp string      `json:"ts"`
		Verb      string      `json:"verb"`
		Files     []trashFile `json:"files"`
	}
	m := trashManifest{
		RunID:     runID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Verb:      "ship",
		Files: []trashFile{
			{
				OrigPath: specDir,
				SavePath: filepath.Join(root, ".forge", snapshotsBaseDir, specSlug),
				Mode:     0o755,
			},
		},
	}
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o600)
}

// snapOnFail is a best-effort snapshot helper called on checkpoint failure.
// It calls TakeSnapshot and silently discards any error (snapshot is advisory).
func snapOnFail(root, slug, checkpoint string) {
	_ = TakeSnapshot(root, slug, checkpoint)
}
