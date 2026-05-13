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

// Package cmdundo implements `forge undo` (M3-22 reversibility contract).
//
// forge undo reverses the last --apply operation by restoring the trash
// snapshot written before the mutation (ADR-024, §17.1 #5).
//
// Every --apply verb must write a snapshot to .forge/trash/<run-id>/ before
// mutating the filesystem. `forge undo` replays the most recent (or named)
// snapshot.
package cmdundo

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 5550..5599).
var (
	ErrUndoFailed = errcode.Register(errcode.Code(5550), "undo failed")
)

const trashDir = ".forge/trash"

// TrashManifest describes one saved snapshot.
type TrashManifest struct {
	RunID     string      `json:"run_id"`
	Timestamp string      `json:"ts"`
	Verb      string      `json:"verb"`
	Files     []TrashFile `json:"files"`
}

// TrashFile is one file captured in the snapshot.
type TrashFile struct {
	OrigPath string `json:"orig_path"`
	SavePath string `json:"save_path"`
	Mode     uint32 `json:"mode"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "undo",
		Summary: "Reverse the last --apply operation by restoring the pre-mutation snapshot (M3, ADR-024).",
		Inputs: []string{
			"--run-id <id>  — specific run to undo (default: latest)",
			"--list         — list available snapshots",
			"--root <path>",
			"--json",
		},
		Outputs:      []string{"stdout: files restored"},
		SideEffects:  []string{"restores files from .forge/trash/<run-id>/"},
		GatesTouched: []string{"ADR-024 reversibility contract"},
		ErrorCodes:   []errcode.Code{ErrUndoFailed},
	})
}

// New returns the cobra command for `forge undo`.
func New() *cobra.Command {
	var (
		root    string
		runID   string
		list    bool
		jsonOut bool
	)
	cmd := &cobra.Command{
		Use:   "undo [--run-id <id>]",
		Short: "Reverse the last --apply operation (M3, ADR-024).",
		Long: "forge undo restores files from the pre-mutation snapshot written by\n" +
			"the last --apply verb call (adopt, fix, generate, eject, upgrade, etc.).\n\n" +
			"Snapshots are stored in .forge/trash/<run-id>/.\n\n" +
			"Use --list to see all available snapshots.\n" +
			"Use --run-id to undo a specific run.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveRoot(root)
			if err != nil {
				return err
			}
			if list {
				return listSnapshots(cmd, r, jsonOut)
			}
			return runUndo(cmd, r, runID, jsonOut)
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "Project root (default: cwd)")
	cmd.Flags().StringVar(&runID, "run-id", "", "Specific run ID to undo (default: latest)")
	cmd.Flags().BoolVar(&list, "list", false, "List available undo snapshots")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON output")
	return cmd
}

func runUndo(cmd *cobra.Command, root, runID string, jsonOut bool) error {
	snapshots, err := listSnapshotManifests(root)
	if err != nil {
		return errcode.New(ErrUndoFailed, "read trash directory", err)
	}
	if len(snapshots) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "undo: no snapshots available")
		return nil
	}
	var target TrashManifest
	if runID == "" {
		target = snapshots[len(snapshots)-1]
	} else {
		for _, s := range snapshots {
			if s.RunID == runID {
				target = s
				break
			}
		}
		if target.RunID == "" {
			return errcode.Newf(ErrUndoFailed, nil, "snapshot %q not found", runID)
		}
	}
	restored := 0
	for _, f := range target.Files {
		src := filepath.Join(root, f.SavePath)
		dst := filepath.Join(root, f.OrigPath)
		if err := restoreFile(src, dst, os.FileMode(f.Mode)); err != nil {
			cmd.PrintErrf("undo: warn: could not restore %s: %v\n", f.OrigPath, err)
			continue
		}
		restored++
	}
	type result struct {
		RunID     string `json:"run_id"`
		Verb      string `json:"verb"`
		Timestamp string `json:"ts"`
		Restored  int    `json:"restored"`
	}
	res := result{RunID: target.RunID, Verb: target.Verb, Timestamp: target.Timestamp, Restored: restored}
	if jsonOut {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "undo: restored %d files from run %s (verb=%s ts=%s)\n",
		restored, target.RunID, target.Verb, target.Timestamp)
	return nil
}

func listSnapshots(cmd *cobra.Command, root string, jsonOut bool) error {
	snapshots, err := listSnapshotManifests(root)
	if err != nil {
		return errcode.New(ErrUndoFailed, "read trash directory", err)
	}
	if jsonOut {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(snapshots)
	}
	if len(snapshots) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "undo list: no snapshots")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%-26s %-12s %-8s %s\n", "RUN ID", "VERB", "FILES", "TIMESTAMP")
	for _, s := range snapshots {
		fmt.Fprintf(cmd.OutOrStdout(), "%-26s %-12s %-8d %s\n", s.RunID, s.Verb, len(s.Files), s.Timestamp)
	}
	return nil
}

func listSnapshotManifests(root string) ([]TrashManifest, error) {
	dir := filepath.Join(root, trashDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var manifests []TrashManifest
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, e.Name(), "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var m TrashManifest
		if err := json.Unmarshal(data, &m); err == nil {
			manifests = append(manifests, m)
		}
	}
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].Timestamp < manifests[j].Timestamp
	})
	return manifests, nil
}

func restoreFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func resolveRoot(root string) (string, error) {
	if root != "" {
		return root, nil
	}
	return os.Getwd()
}

// WriteTrashSnapshot saves a pre-mutation snapshot so `forge undo` can replay it.
// Called by every --apply verb before mutating the filesystem (§17.1 #5, ADR-024).
func WriteTrashSnapshot(root, verb string, origPaths []string) (string, error) {
	runID := fmt.Sprintf("%s-%d", verb, time.Now().UnixMicro())
	snapDir := filepath.Join(root, trashDir, runID)
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		return "", err
	}
	var files []TrashFile
	for _, p := range origPaths {
		abs := filepath.Join(root, p)
		info, err := os.Stat(abs)
		if err != nil {
			continue // file doesn't exist yet; skip
		}
		relSave := filepath.Join(runID, filepath.Base(p))
		savePath := filepath.Join(snapDir, filepath.Base(p))
		if err := copyFile(abs, savePath, info.Mode()); err != nil {
			continue
		}
		files = append(files, TrashFile{
			OrigPath: p,
			SavePath: filepath.Join(trashDir, relSave),
			Mode:     uint32(info.Mode()),
		})
	}
	manifest := TrashManifest{
		RunID:     runID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Verb:      verb,
		Files:     files,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	return runID, os.WriteFile(filepath.Join(snapDir, "manifest.json"), data, 0o644)
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
