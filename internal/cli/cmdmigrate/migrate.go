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

// Package cmdmigrate implements `forge migrate` (DEV-M1-50, spec §4).
//
// Manages database schema migrations. The migration engine uses the filesystem
// as the source of truth: SQL files in the migrations/ directory (or the path
// set in .forge/conventions.json#migrations_dir).
//
// Sub-commands:
//
//	up [N]     — apply N pending migrations (default: all)
//	down [N]   — roll back N applied migrations (default: 1; requires --allow-irreversible)
//	status     — list applied/pending migrations
//	suggest    — suggest a new migration from model changes (LLM-assisted)
//	repair     — fix checksum mismatches in migration history
//
// Migration history is stored in .forge/migrations/history.json.
// The database driver adapters are in M2; the M1 implementation exercises the
// full file-system layer and planning logic independently of a live DB.
package cmdmigrate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 1800..1899).
var (
	ErrMigrateFailed = errcode.Register(errcode.Code(1800), "migration failed")
)

// MigrationEntry records one migration's state.
type MigrationEntry struct {
	Version   string `json:"version"`
	Name      string `json:"name"`
	Applied   bool   `json:"applied"`
	AppliedAt string `json:"applied_at,omitempty"`
	Checksum  string `json:"checksum,omitempty"`
}

// MigrateResult summarises an up/down/status operation.
type MigrateResult struct {
	Operation string           `json:"operation"`
	Applied   []MigrationEntry `json:"applied"`
	Pending   []MigrationEntry `json:"pending"`
	Rolled    []MigrationEntry `json:"rolled_back,omitempty"`
	DryRun    bool             `json:"dry_run"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "migrate",
		Summary: "Manage database schema migrations (DEV-M1-50, spec §4).",
		Inputs: []string{
			"up [N]              — apply N pending migrations (default: all)",
			"down [N]            — roll back N (default: 1; requires --allow-irreversible)",
			"status              — list applied / pending migrations",
			"suggest             — LLM-assisted new migration suggestion",
			"repair              — fix checksum mismatches",
			"--dir <path>        — migrations directory (default: migrations/)",
			"--dry-run           — plan only; do not modify history",
			"--allow-irreversible — allow destructive down migrations",
			"--json              — machine-readable output",
		},
		Outputs:      []string{"stdout: migration status or confirmation"},
		SideEffects:  []string{"up/down: updates .forge/migrations/history.json; DB writes in M2"},
		GatesTouched: []string{"§4 migrate", "§16.5.4 #1 data safety", "ADR-024 reversibility"},
		ErrorCodes:   []errcode.Code{ErrMigrateFailed},
	})
}

// New returns the cobra command for `forge migrate`.
func New() *cobra.Command {
	var (
		root              string
		migDir            string
		dryRun            bool
		allowIrreversible bool
		asJSON            bool
	)
	cmd := &cobra.Command{
		Use:   "migrate <up|down|status|suggest|repair>",
		Short: "Manage database schema migrations.",
		Long: "forge migrate manages the database schema migration lifecycle.\n\n" +
			"Sub-commands:\n" +
			"  up [N]   — apply N pending migrations (default: all)\n" +
			"  down [N] — roll back N applied migrations (default: 1)\n" +
			"  status   — list applied and pending migrations\n" +
			"  suggest  — suggest a new migration (LLM-assisted)\n" +
			"  repair   — fix migration checksum mismatches\n\n" +
			"Migration files live in migrations/ (override with --dir).\n" +
			"History tracked in .forge/migrations/history.json.\n\n" +
			"NOTE: Live DB execution requires DATABASE_URL; M1 exercises the\n" +
			"      planning layer; actual DB writes arrive in M2.",
	}

	makeRunE := func(op string) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrMigrateFailed, "getwd", err)
				}
				root = cwd
			}
			if migDir == "" {
				migDir = filepath.Join(root, "migrations")
			}
			switch op {
			case "down":
				if !allowIrreversible && !dryRun {
					return errcode.Newf(ErrMigrateFailed, nil,
						"migrate down requires --allow-irreversible (ADR-024 reversibility contract)")
				}
			}
			result, err := runOp(root, migDir, op, args, dryRun)
			if err != nil {
				return errcode.New(ErrMigrateFailed, op, err)
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			renderText(cmd, result)
			return nil
		}
	}

	upCmd := &cobra.Command{
		Use:   "up [N]",
		Short: "Apply N pending migrations (default: all).",
		Args:  cobra.MaximumNArgs(1),
		RunE:  makeRunE("up"),
	}

	downCmd := &cobra.Command{
		Use:   "down [N]",
		Short: "Roll back N applied migrations (default: 1). Requires --allow-irreversible.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  makeRunE("down"),
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "List applied and pending migrations.",
		Args:  cobra.NoArgs,
		RunE:  makeRunE("status"),
	}

	suggestCmd := &cobra.Command{
		Use:   "suggest",
		Short: "Suggest a new migration from model changes (LLM-assisted).",
		Args:  cobra.NoArgs,
		RunE:  makeRunE("suggest"),
	}

	repairCmd := &cobra.Command{
		Use:   "repair",
		Short: "Fix migration checksum mismatches.",
		Args:  cobra.NoArgs,
		RunE:  makeRunE("repair"),
	}

	for _, sub := range []*cobra.Command{upCmd, downCmd, statusCmd, suggestCmd, repairCmd} {
		sub.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
		sub.Flags().StringVar(&migDir, "dir", "", "migrations directory (default: migrations/)")
		sub.Flags().BoolVar(&dryRun, "dry-run", false, "plan only; do not modify history")
		sub.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	}
	downCmd.Flags().BoolVar(&allowIrreversible, "allow-irreversible", false,
		"required to execute migrate down (ADR-024)")

	cmd.AddCommand(upCmd, downCmd, statusCmd, suggestCmd, repairCmd)
	return cmd
}

// historyPath returns the path to the migration history file.
func historyPath(root string) string {
	return filepath.Join(root, ".forge", "migrations", "history.json")
}

// loadHistory reads the migration history.
func loadHistory(root string) []MigrationEntry {
	data, err := os.ReadFile(historyPath(root))
	if err != nil {
		return nil
	}
	var entries []MigrationEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	return entries
}

// saveHistory writes the migration history.
func saveHistory(root string, entries []MigrationEntry) error {
	dir := filepath.Dir(historyPath(root))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(historyPath(root), data, 0o644)
}

// scanMigrations lists all SQL migration files in migDir.
func scanMigrations(migDir string) ([]MigrationEntry, error) {
	entries, err := os.ReadDir(migDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var migs []MigrationEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".up.sql")
		parts := strings.SplitN(base, "_", 2)
		version := parts[0]
		name := ""
		if len(parts) > 1 {
			name = parts[1]
		}
		migs = append(migs, MigrationEntry{Version: version, Name: name})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].Version < migs[j].Version })
	return migs, nil
}

// runOp executes the named migration operation.
func runOp(root, migDir, op string, args []string, dryRun bool) (MigrateResult, error) {
	result := MigrateResult{Operation: op, DryRun: dryRun}

	history := loadHistory(root)
	appliedSet := make(map[string]MigrationEntry)
	for _, h := range history {
		if h.Applied {
			appliedSet[h.Version] = h
		}
	}

	allMigs, err := scanMigrations(migDir)
	if err != nil {
		return result, err
	}

	// Build applied / pending lists.
	for _, m := range allMigs {
		if h, ok := appliedSet[m.Version]; ok {
			m.Applied = true
			m.AppliedAt = h.AppliedAt
			result.Applied = append(result.Applied, m)
		} else {
			result.Pending = append(result.Pending, m)
		}
	}

	switch op {
	case "status":
		// Nothing more to do.

	case "up":
		limit := len(result.Pending)
		if len(args) == 1 {
			fmt.Sscanf(args[0], "%d", &limit) //nolint:errcheck
		}
		toApply := result.Pending
		if limit < len(toApply) {
			toApply = toApply[:limit]
		}
		if !dryRun {
			for _, m := range toApply {
				m.Applied = true
				m.AppliedAt = time.Now().UTC().Format(time.RFC3339)
				// Append to history.
				history = append(history, m)
			}
			if err := saveHistory(root, history); err != nil {
				return result, err
			}
		}

	case "down":
		limit := 1
		if len(args) == 1 {
			fmt.Sscanf(args[0], "%d", &limit) //nolint:errcheck
		}
		if limit > len(result.Applied) {
			limit = len(result.Applied)
		}
		// Roll back in reverse order.
		toRoll := result.Applied[len(result.Applied)-limit:]
		if !dryRun {
			rolledVersions := make(map[string]bool, len(toRoll))
			for _, m := range toRoll {
				rolledVersions[m.Version] = true
			}
			var remaining []MigrationEntry
			for _, h := range history {
				if !rolledVersions[h.Version] {
					remaining = append(remaining, h)
				}
			}
			if err := saveHistory(root, remaining); err != nil {
				return result, err
			}
		}
		result.Rolled = toRoll

	case "suggest":
		result.Operation = "suggest"
		// Naming convention hint — LLM wiring in M2.

	case "repair":
		result.Operation = "repair"
		// Checksum repair — wiring in M2.
	}

	return result, nil
}

func renderText(cmd *cobra.Command, r MigrateResult) {
	w := cmd.OutOrStdout()
	mode := ""
	if r.DryRun {
		mode = " [dry-run]"
	}
	fmt.Fprintf(w, "forge migrate %s%s\n\n", r.Operation, mode)

	if r.Operation == "suggest" {
		fmt.Fprintln(w, "  LLM-assisted migration suggestion will be available in M2.")
		fmt.Fprintln(w, "  Tip: describe your model changes to an AI and paste the SQL into migrations/.")
		return
	}
	if r.Operation == "repair" {
		fmt.Fprintln(w, "  Checksum repair will be available in M2 (requires live DB connection).")
		return
	}

	fmt.Fprintf(w, "  Applied  (%d):\n", len(r.Applied))
	for _, m := range r.Applied {
		fmt.Fprintf(w, "    ✓ %s %s (at %s)\n", m.Version, m.Name, m.AppliedAt)
	}
	fmt.Fprintf(w, "  Pending  (%d):\n", len(r.Pending))
	for _, m := range r.Pending {
		fmt.Fprintf(w, "    ○ %s %s\n", m.Version, m.Name)
	}
	if len(r.Rolled) > 0 {
		fmt.Fprintf(w, "  Rolled back (%d):\n", len(r.Rolled))
		for _, m := range r.Rolled {
			fmt.Fprintf(w, "    ↩ %s %s\n", m.Version, m.Name)
		}
	}
}
