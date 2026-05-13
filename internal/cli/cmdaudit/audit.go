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
// Package cmdaudit implements `forge audit` — append/verify/show
// the tamper-evident action ledger (.forge/audit.log).
package cmdaudit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/audit"
	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 3400..3499).
var (
	ErrAuditFailed   = errcode.Register(errcode.Code(3400), "audit operation failed")
	ErrAuditTampered = errcode.Register(errcode.Code(3401), "audit ledger tampered or corrupted")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "audit",
		Summary: "Inspect or verify the tamper-evident action ledger (.forge/audit.log).",
		Inputs: []string{
			"<subcommand>: show | verify | append (require '--verb' '--action')",
			"--root <path> (default cwd)",
			"--json",
		},
		Outputs:      []string{"stdout: ledger entries or verification result"},
		SideEffects:  []string{"`append` writes a chained entry to the ledger"},
		GatesTouched: []string{"§16.5.2 dogfood (audit trail)", "§16.5.4 #6 — auditability"},
		ErrorCodes:   []errcode.Code{ErrAuditFailed, ErrAuditTampered},
	})
}

// New returns the cobra command.
func New() *cobra.Command {
	var (
		root   string
		asJSON bool
		verb   string
		action string
	)
	cmd := &cobra.Command{
		Use:   "audit <show|verify|append>",
		Short: "Inspect / verify / append to the action ledger.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrAuditFailed, "getwd", err)
				}
				root = cwd
			}
			path := filepath.Join(root, audit.DefaultPath)
			ledger, err := audit.Open(path)
			if err != nil {
				return errcode.New(ErrAuditFailed, "open ledger", err)
			}
			switch args[0] {
			case "show":
				return showLedger(cmd, ledger, asJSON)
			case "verify":
				return verifyLedger(cmd, ledger, asJSON)
			case "append":
				if verb == "" || action == "" {
					return errcode.New(ErrAuditFailed, "append requires --verb and --action", nil)
				}
				return appendLedger(cmd, ledger, verb, action, asJSON)
			default:
				return errcode.Newf(ErrAuditFailed, nil, "unknown subcommand %q", args[0])
			}
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	cmd.Flags().StringVar(&verb, "verb", "", "(append) verb name")
	cmd.Flags().StringVar(&action, "action", "", "(append) action name")
	cmd.AddCommand(newQueryCmd())
	cmd.AddCommand(newFailureRegisterCmd())
	cmd.AddCommand(newExportCmd())
	cmd.AddCommand(newEraseCmd())
	return cmd
}

func showLedger(cmd *cobra.Command, l *audit.Ledger, asJSON bool) error {
	entries, err := l.All()
	if err != nil {
		return errcode.New(ErrAuditFailed, "read ledger", err)
	}
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "audit ledger: %d entries\n", len(entries))
	for i, e := range entries {
		fmt.Fprintf(cmd.OutOrStdout(), "  #%d %s %s/%s by=%s hash=%s\n",
			i, e.Timestamp.Format("2006-01-02T15:04:05Z"), e.Verb, e.Action, e.Actor, short(e.Hash))
	}
	return nil
}

func verifyLedger(cmd *cobra.Command, l *audit.Ledger, asJSON bool) error {
	idx, err := l.Verify()
	out := map[string]any{"intact": idx == -1, "broken_index": idx}
	switch {
	case asJSON:
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
	case idx == -1:
		fmt.Fprintln(cmd.OutOrStdout(), "audit: chain intact ✓")
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "audit: chain BROKEN at #%d (%v)\n", idx, err)
	}
	if err != nil {
		return errcode.Newf(ErrAuditTampered, err, "verify failed at #%d", idx)
	}
	return nil
}

func appendLedger(cmd *cobra.Command, l *audit.Ledger, verb, action string, asJSON bool) error {
	e, err := l.Append(audit.Entry{Verb: verb, Action: action, Actor: os.Getenv("USER")})
	if err != nil {
		return errcode.New(ErrAuditFailed, "append", err)
	}
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(e)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "audit appended: %s/%s hash=%s\n", e.Verb, e.Action, short(e.Hash))
	return nil
}

func short(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

// newExportCmd implements `forge audit export` (spec §4 audit sub-verb).
// Writes the full ledger to a JSON or CSV file for compliance hand-off.
func newExportCmd() *cobra.Command {
	var (
		root   string
		output string
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the audit ledger to a file (JSON, for compliance/GDPR hand-off).",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrAuditFailed, "getwd", err)
				}
				root = cwd
			}
			path := filepath.Join(root, audit.DefaultPath)
			ledger, err := audit.Open(path)
			if err != nil {
				return errcode.New(ErrAuditFailed, "open ledger", err)
			}
			entries, err := ledger.All()
			if err != nil {
				return errcode.New(ErrAuditFailed, "read ledger", err)
			}
			data, err := json.MarshalIndent(entries, "", "  ")
			if err != nil {
				return errcode.New(ErrAuditFailed, "marshal ledger", err)
			}
			data = append(data, '\n')
			if output == "" || output == "-" {
				_, err = cmd.OutOrStdout().Write(data)
				return err
			}
			if err := os.WriteFile(output, data, 0o600); err != nil {
				return errcode.Newf(ErrAuditFailed, err, "write export file %s", output)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "audit: exported %d entries to %s\n", len(entries), output)
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().StringVar(&output, "output", "-", "output file path (default: stdout)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON (default for export)")
	return cmd
}

// newEraseCmd implements `forge audit erase <subject>` (spec §4 audit sub-verb).
// Supports GDPR right-to-erasure by redacting entries for a given actor/subject.
// NOTE: In M1 this is a stub that prints what would be erased; full implementation
// requires immutable-log compaction and is scheduled for M2.
func newEraseCmd() *cobra.Command {
	var (
		root   string
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:   "erase <subject>",
		Short: "GDPR right-to-erasure: redact audit entries for a given actor (M2 full impl).",
		Long: "Finds all audit entries where the actor matches <subject> and marks them\n" +
			"as redacted. In M1 (--dry-run default) only lists what would be erased.\n\n" +
			"Full compaction of the immutable log is scheduled for M2.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			subject := args[0]
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrAuditFailed, "getwd", err)
				}
				root = cwd
			}
			path := filepath.Join(root, audit.DefaultPath)
			ledger, err := audit.Open(path)
			if err != nil {
				return errcode.New(ErrAuditFailed, "open ledger", err)
			}
			entries, err := ledger.All()
			if err != nil {
				return errcode.New(ErrAuditFailed, "read ledger", err)
			}
			var matches int
			for _, e := range entries {
				if e.Actor == subject {
					matches++
					fmt.Fprintf(cmd.OutOrStdout(), "  would erase: #%s %s %s/%s\n",
						e.Hash[:12], e.Timestamp.Format("2006-01-02T15:04:05Z"), e.Verb, e.Action)
				}
			}
			if matches == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "audit erase: no entries found for subject %q\n", subject)
				return nil
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "audit erase [dry-run]: would redact %d entries for %q\n", matches, subject)
				fmt.Fprintln(cmd.OutOrStdout(), "re-run with --dry-run=false to apply (M2 full implementation)")
				return nil
			}
			// M1 stub: full log compaction is M2
			return errcode.Newf(ErrAuditFailed, nil,
				"forge audit erase: full log compaction scheduled for M2; run with --dry-run to preview")
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "preview what would be erased without modifying the ledger")
	return cmd
}
