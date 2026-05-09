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
