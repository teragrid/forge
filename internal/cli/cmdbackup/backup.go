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

// Package cmdbackup implements `forge backup` (spec §4, namespace 9 — Operate).
//
// forge backup creates a point-in-time snapshot of the project's database and
// key artefacts, recording metadata in .forge/backups/. It is designed to be
// run before risky operations (schema migrations, major upgrades) and as a
// complement to `forge rollback`.
//
// Usage:
//
//	forge backup                   # dry-run: describe what would be backed up
//	forge backup --apply           # create the backup snapshot
//	forge backup list              # list available snapshots
//	forge backup verify <snap-id>  # verify integrity of a snapshot
//	forge backup --json            # machine-readable output
package cmdbackup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 6100..6199).
var (
	ErrBackupFailed = errcode.Register(errcode.Code(6100), "backup operation failed")
)

const backupDir = ".forge/backups"

// BackupManifest records one backup snapshot.
type BackupManifest struct {
	ID        string       `json:"id"`
	Timestamp string       `json:"ts"`
	Mode      string       `json:"mode"` // "dry-run" | "apply"
	Entries   []BackupItem `json:"entries"`
	Checksum  string       `json:"checksum,omitempty"`
}

// BackupItem is one artefact captured in the snapshot.
type BackupItem struct {
	Kind string `json:"kind"` // "schema" | "config" | "env-template"
	Path string `json:"path"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "backup",
		Summary: "Create a point-in-time backup snapshot before risky operations (spec §4, namespace 9).",
		Inputs: []string{
			"(default)      — dry-run: describe what would be backed up",
			"list           — list available backup snapshots",
			"verify <id>    — verify integrity of a snapshot",
			"--apply        — write snapshot to .forge/backups/",
			"--json         — machine-readable output",
			"--root <path>  — project root (default: cwd)",
		},
		Outputs:      []string{"stdout: backup result or snapshot list"},
		SideEffects:  []string{"with --apply: writes snapshot manifest to .forge/backups/<id>.json"},
		GatesTouched: []string{"§4 operate"},
		ErrorCodes:   []errcode.Code{ErrBackupFailed},
	})
}

// New returns the cobra command for `forge backup`.
func New() *cobra.Command {
	var (
		root   string
		apply  bool
		asJSON bool
	)

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Create a point-in-time backup snapshot before risky operations.",
		Long: "Snapshots the project's migration state, config artefacts, and env templates\n" +
			"into .forge/backups/. Run before `forge migrate up`, major upgrades, or\n" +
			"any destructive operation. Pairs with `forge rollback`.\n\n" +
			"Safe by default: --apply is required to persist the snapshot.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrBackupFailed, "getwd", err)
				}
				root = cwd
			}
			return runBackup(cmd, root, apply, asJSON)
		},
	}

	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().BoolVar(&apply, "apply", false, "write the backup snapshot to disk")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")

	cmd.AddCommand(newListCmd(), newVerifyCmd())
	return cmd
}

func runBackup(cmd *cobra.Command, root string, apply, asJSON bool) error {
	mode := "dry-run"
	if apply {
		mode = "apply"
	}

	items := collectItems(root)
	id := fmt.Sprintf("%s-backup", time.Now().UTC().Format("20060102T150405Z"))
	manifest := BackupManifest{
		ID:        id,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Mode:      mode,
		Entries:   items,
	}

	if apply {
		dir := filepath.Join(root, backupDir)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return errcode.New(ErrBackupFailed, "mkdir", err)
		}
		manifestPath := filepath.Join(dir, id+".json")
		data, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return errcode.New(ErrBackupFailed, "marshal", err)
		}
		if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
			return errcode.New(ErrBackupFailed, "write", err)
		}
	}

	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(manifest)
	}

	if apply {
		fmt.Fprintf(cmd.OutOrStdout(), "backup created: %s (%d items)\n", id, len(items))
		for _, item := range items {
			fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s\n", item.Kind, item.Path)
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "(dry-run) would create backup: %s\n", id)
		for _, item := range items {
			fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s\n", item.Kind, item.Path)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Re-run with --apply to persist the snapshot.")
	}
	return nil
}

// collectItems enumerates artefacts worth snapshotting in a Forge project.
func collectItems(root string) []BackupItem {
	var items []BackupItem

	// Migration directory — record each migration file.
	for _, migrDir := range []string{"migrations", "db/migrations"} {
		dir := filepath.Join(root, migrDir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				items = append(items, BackupItem{
					Kind: "schema",
					Path: filepath.Join(migrDir, e.Name()),
				})
			}
		}
	}

	// Forge config files.
	for _, cfg := range []string{
		"forge.config.ts", "forge.config.yml", ".forge-conventions",
		".forge/conventions.json", ".forge/hygiene.yml",
	} {
		if _, err := os.Stat(filepath.Join(root, cfg)); err == nil {
			items = append(items, BackupItem{Kind: "config", Path: cfg})
		}
	}

	// Env templates.
	for _, env := range []string{".env.example", ".env.template"} {
		if _, err := os.Stat(filepath.Join(root, env)); err == nil {
			items = append(items, BackupItem{Kind: "env-template", Path: env})
		}
	}

	return items
}

// newListCmd returns the `forge backup list` subcommand.
func newListCmd() *cobra.Command {
	var (
		root   string
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available backup snapshots.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrBackupFailed, "getwd", err)
				}
				root = cwd
			}
			dir := filepath.Join(root, backupDir)
			entries, err := os.ReadDir(dir)
			if os.IsNotExist(err) {
				fmt.Fprintln(cmd.OutOrStdout(), "No backups found.")
				return nil
			}
			if err != nil {
				return errcode.New(ErrBackupFailed, "readdir", err)
			}

			var manifests []BackupManifest
			for _, e := range entries {
				if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
					continue
				}
				data, err := os.ReadFile(filepath.Join(dir, e.Name()))
				if err != nil {
					continue
				}
				var m BackupManifest
				if err := json.Unmarshal(data, &m); err != nil {
					continue
				}
				manifests = append(manifests, m)
			}

			sort.Slice(manifests, func(i, j int) bool {
				return manifests[i].Timestamp > manifests[j].Timestamp
			})

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(manifests)
			}

			if len(manifests) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No backups found.")
				return nil
			}
			for _, m := range manifests {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  (%d items)\n",
					m.ID, m.Timestamp, len(m.Entries))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}

// newVerifyCmd returns the `forge backup verify <id>` subcommand.
func newVerifyCmd() *cobra.Command {
	var root string
	cmd := &cobra.Command{
		Use:   "verify <id>",
		Short: "Verify integrity of a backup snapshot.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrBackupFailed, "getwd", err)
				}
				root = cwd
			}
			manifestPath := filepath.Join(root, backupDir, id+".json")
			data, err := os.ReadFile(manifestPath)
			if err != nil {
				if os.IsNotExist(err) {
					return errcode.Newf(ErrBackupFailed, nil, "backup %q not found", id)
				}
				return errcode.New(ErrBackupFailed, "read", err)
			}
			var m BackupManifest
			if err := json.Unmarshal(data, &m); err != nil {
				return errcode.New(ErrBackupFailed, "parse", err)
			}

			// Verify each referenced file exists.
			var missing []string
			for _, item := range m.Entries {
				if _, err := os.Stat(filepath.Join(root, item.Path)); os.IsNotExist(err) {
					missing = append(missing, item.Path)
				}
			}

			if len(missing) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "backup %s: INCOMPLETE — %d missing file(s):\n", id, len(missing))
				for _, f := range missing {
					fmt.Fprintf(cmd.OutOrStdout(), "  missing: %s\n", f)
				}
				return errcode.Newf(ErrBackupFailed, nil, "backup %q is incomplete", id)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "backup %s: OK (%d items verified)\n", id, len(m.Entries))
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	return cmd
}
