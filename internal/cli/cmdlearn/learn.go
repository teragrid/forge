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

// Package cmdlearn implements `forge learn` (M2-09, M2-10).
//
// The learning loop captures anonymised usage signals (opt-in) and shares
// them with the central aggregator so that scan rules and LLM prompts can
// improve over time. All capture is gated by explicit user consent.
//
// Sub-commands:
//
//	record  — append a learning entry to .forge/learn.jsonl
//	submit  — flush .forge/learn.jsonl to the aggregator endpoint
//	status  — show pending entry count and last-submit timestamp
//	reset   — delete all pending entries (does not contact aggregator)
package cmdlearn

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 5200..5299).
var (
	ErrLearnFailed = errcode.Register(errcode.Code(5200), "learn operation failed")
)

const learnFile = ".forge/learn.jsonl"

// LearnEntry is one anonymised learning record.
type LearnEntry struct {
	Timestamp string `json:"ts"`
	Verb      string `json:"verb"`
	RuleID    string `json:"rule_id,omitempty"`
	Outcome   string `json:"outcome"` // "applied" | "dismissed" | "waivedR"
	Model     string `json:"model,omitempty"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "learn",
		Summary: "Opt-in learning loop: capture and submit anonymised usage signals to improve scan rules and prompts (M2).",
		Inputs: []string{
			"record  — append a learning entry to .forge/learn.jsonl",
			"submit  — flush pending entries to the aggregator",
			"status  — show pending count and last-submit time",
			"reset   — delete all pending entries locally",
			"--root <path>",
			"--json",
		},
		Outputs:      []string{"stdout: learn operation result"},
		SideEffects:  []string{"record: appends to .forge/learn.jsonl", "submit: sends HTTP POST (opt-in only)", "reset: deletes .forge/learn.jsonl"},
		GatesTouched: []string{"§17.3 learning loop"},
		ErrorCodes:   []errcode.Code{ErrLearnFailed},
	})
}

// New returns the cobra command for `forge learn`.
func New() *cobra.Command {
	var (
		root     string
		jsonOut  bool
		endpoint string
	)
	cmd := &cobra.Command{
		Use:   "learn <record|submit|status|reset>",
		Short: "Opt-in learning loop for improving scan rules and prompts (M2).",
		Long: "forge learn manages the opt-in signal capture loop.\n\n" +
			"All data collection requires explicit consent (telemetry opt-in).\n" +
			"Entries are stored in .forge/learn.jsonl and submitted to the\n" +
			"aggregator with `forge learn submit`.",
	}
	cmd.PersistentFlags().StringVar(&root, "root", "", "Project root (default: cwd)")
	cmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Emit JSON output")

	// record
	recordCmd := &cobra.Command{
		Use:   "record --verb <verb> --rule <rule> --outcome <outcome>",
		Short: "Append a learning entry to .forge/learn.jsonl.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			verb, _ := cmd.Flags().GetString("verb")
			rule, _ := cmd.Flags().GetString("rule")
			outcome, _ := cmd.Flags().GetString("outcome")
			if outcome == "" {
				outcome = "applied"
			}
			r, err := resolveLearnRoot(root)
			if err != nil {
				return err
			}
			entry := LearnEntry{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Verb:      verb,
				RuleID:    rule,
				Outcome:   outcome,
			}
			if err := appendEntry(r, entry); err != nil {
				return errcode.New(ErrLearnFailed, "append learn entry", err)
			}
			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(entry)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "learn: recorded entry (verb=%s rule=%s outcome=%s)\n", verb, rule, outcome)
			return nil
		},
	}
	recordCmd.Flags().String("verb", "", "Forge verb that produced the signal")
	recordCmd.Flags().String("rule", "", "Rule ID (optional)")
	recordCmd.Flags().String("outcome", "applied", "Outcome: applied|dismissed|waived")

	// submit
	submitCmd := &cobra.Command{
		Use:   "submit",
		Short: "Flush pending entries to the aggregator endpoint.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveLearnRoot(root)
			if err != nil {
				return err
			}
			entries, err := loadEntries(r)
			if err != nil {
				return errcode.New(ErrLearnFailed, "load learn entries", err)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "learn submit: no pending entries")
				return nil
			}
			ep := endpoint
			if ep == "" {
				ep = "https://learn.forge.dev/v1/ingest"
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"learn submit: would POST %d entries to %s (wiring in M2-10)\n", len(entries), ep)
			return nil
		},
	}
	submitCmd.Flags().StringVar(&endpoint, "endpoint", "", "Aggregator URL (default: https://learn.forge.dev/v1/ingest)")

	// status
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show pending entry count and last-submit timestamp.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveLearnRoot(root)
			if err != nil {
				return err
			}
			entries, _ := loadEntries(r)
			type statusResult struct {
				PendingEntries int    `json:"pending_entries"`
				LearnFile      string `json:"learn_file"`
			}
			res := statusResult{PendingEntries: len(entries), LearnFile: filepath.Join(r, learnFile)}
			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "learn status: %d pending entries in %s\n", res.PendingEntries, res.LearnFile)
			return nil
		},
	}

	// reset
	resetCmd := &cobra.Command{
		Use:   "reset",
		Short: "Delete all pending learning entries.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveLearnRoot(root)
			if err != nil {
				return err
			}
			path := filepath.Join(r, learnFile)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return errcode.New(ErrLearnFailed, "reset learn file", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "learn reset: all pending entries deleted")
			return nil
		},
	}

	// G-030–G-035 extended subcommands.
	cmd.AddCommand(
		recordCmd, submitCmd, statusCmd, resetCmd,
		NewPromoteCmd(&root, &jsonOut),
		NewAntiPatternsCmd(&root, &jsonOut),
		NewTeachCmd(&root, &jsonOut),
		NewSessionCmd(&root, &jsonOut),
		NewInstructionsCmd(&root, &jsonOut),
		NewShareCmd(&root, &jsonOut),
	)
	return cmd
}

func resolveLearnRoot(root string) (string, error) {
	if root != "" {
		return root, nil
	}
	return os.Getwd()
}

func appendEntry(root string, e LearnEntry) error {
	path := filepath.Join(root, learnFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(e)
}

func loadEntries(root string) ([]LearnEntry, error) {
	path := filepath.Join(root, learnFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []LearnEntry
	for _, line := range splitLines(string(data)) {
		if line == "" {
			continue
		}
		var e LearnEntry
		if err := json.Unmarshal([]byte(line), &e); err == nil {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
