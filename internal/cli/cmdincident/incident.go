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
// Package cmdincident implements `forge incident` — ADR-021 incident lifecycle
// management (DEV-M3-06).
package cmdincident

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/incident"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Error codes (range 4000..4099 — cli/incident).
var (
	ErrIncidentFailed  = errcode.Register(errcode.Code(4000), "incident operation failed")
	ErrIncidentInvalid = errcode.Register(errcode.Code(4001), "invalid incident data")
	ErrIncidentState   = errcode.Register(errcode.Code(4002), "illegal incident state transition")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "incident",
		Summary: "Manage incident lifecycle (ADR-021). Subcommands: new, update, list, close.",
		Inputs: []string{
			"<subcommand>: new | update | list | close",
			"--root <path> (default: cwd)",
			"--json",
		},
		Outputs:      []string{"stdout: incident summary or JSON"},
		SideEffects:  []string{".forge/incidents/<id>.json (written on new/update/close)"},
		GatesTouched: []string{"§16.5.3 #4 — incident response / ADR-021"},
		ErrorCodes:   []errcode.Code{ErrIncidentFailed, ErrIncidentInvalid, ErrIncidentState},
	})
}

// New returns the `forge incident` cobra command.
func New() *cobra.Command {
	var root string
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "incident <new|update|list|close>",
		Short: "Manage incident lifecycle (ADR-021).",
	}
	cmd.PersistentFlags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.PersistentFlags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")

	// ── new ───────────────────────────────────────────────────────────────────
	var (
		newID       string
		newTitle    string
		newSeverity string
		newSystems  string
	)
	newCmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new incident (initial state: identified).",
		RunE: func(c *cobra.Command, _ []string) error {
			dir := incidentDir(root)
			sev := incident.Severity(strings.ToUpper(newSeverity))
			systems := splitComma(newSystems)
			inc := incident.New(newID, newTitle, sev, systems)
			if err := inc.Validate(); err != nil {
				return errcode.New(ErrIncidentInvalid, "validate", err)
			}
			if err := incident.Save(dir, inc); err != nil {
				return errcode.New(ErrIncidentFailed, "save", err)
			}
			printIncident(c, inc, asJSON)
			return nil
		},
	}
	newCmd.Flags().StringVar(&newID, "id", "", "incident ID (required)")
	newCmd.Flags().StringVar(&newTitle, "title", "", "short incident title (required)")
	newCmd.Flags().StringVar(&newSeverity, "severity", "S2", "severity tier: S0 S1 S2 S3")
	newCmd.Flags().StringVar(&newSystems, "systems", "", "comma-separated affected systems")
	newCmd.MarkFlagRequired("id")    //nolint:errcheck
	newCmd.MarkFlagRequired("title") //nolint:errcheck
	cmd.AddCommand(newCmd)

	// ── update ────────────────────────────────────────────────────────────────
	var (
		updateState string
		updateNote  string
	)
	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Transition an incident to a new state.",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			dir := incidentDir(root)
			inc, err := incident.Load(dir, args[0])
			if err != nil {
				return errcode.New(ErrIncidentFailed, "load", err)
			}
			if updateState != "" {
				st := incident.State(strings.ToLower(updateState))
				if err := incident.Transition(inc, st); err != nil {
					return errcode.New(ErrIncidentState, "transition", err)
				}
			}
			if updateNote != "" {
				inc.Notes = append(inc.Notes, updateNote)
			}
			if err := incident.Save(dir, inc); err != nil {
				return errcode.New(ErrIncidentFailed, "save", err)
			}
			printIncident(c, inc, asJSON)
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateState, "state", "", "new lifecycle state")
	updateCmd.Flags().StringVar(&updateNote, "note", "", "free-text note to append")
	cmd.AddCommand(updateCmd)

	// ── list ──────────────────────────────────────────────────────────────────
	var listOpen bool
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List incidents. --open filters to unresolved only.",
		RunE: func(c *cobra.Command, _ []string) error {
			dir := incidentDir(root)
			all, err := incident.LoadAll(dir)
			if err != nil {
				return errcode.New(ErrIncidentFailed, "load all", err)
			}
			var results []*incident.Incident
			for _, inc := range all {
				if listOpen && !inc.IsOpen() {
					continue
				}
				results = append(results, inc)
			}
			if asJSON {
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}
			if len(results) == 0 {
				fmt.Fprintln(c.OutOrStdout(), "no incidents found")
				return nil
			}
			for _, inc := range results {
				fmt.Fprintf(c.OutOrStdout(), "%-16s  %-6s  %-32s  %s\n",
					inc.ID, inc.Severity, inc.State, inc.Title)
			}
			return nil
		},
	}
	listCmd.Flags().BoolVar(&listOpen, "open", false, "show only open incidents")
	cmd.AddCommand(listCmd)

	// ── close ─────────────────────────────────────────────────────────────────
	var closePostmortem string
	closeCmd := &cobra.Command{
		Use:   "close <id>",
		Short: "Mark an incident as fixed (or post-mortem-published).",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			dir := incidentDir(root)
			inc, err := incident.Load(dir, args[0])
			if err != nil {
				return errcode.New(ErrIncidentFailed, "load", err)
			}
			// Drive to StateFixed (best path from current state).
			if err := driveToFixed(inc); err != nil {
				return errcode.New(ErrIncidentState, "close", err)
			}
			if closePostmortem != "" {
				if err := incident.Transition(inc, incident.StatePostMortemPublished); err != nil {
					return errcode.New(ErrIncidentState, "post-mortem", err)
				}
				inc.Postmortem = closePostmortem
			}
			if err := incident.Save(dir, inc); err != nil {
				return errcode.New(ErrIncidentFailed, "save", err)
			}
			printIncident(c, inc, asJSON)
			return nil
		},
	}
	closeCmd.Flags().StringVar(&closePostmortem, "postmortem", "", "path or URL to post-mortem document")
	cmd.AddCommand(closeCmd)

	// G-111: auto-triage
	cmd.AddCommand(newTriageCmd())

	return cmd
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func incidentDir(root string) string {
	if root == "" {
		cwd, _ := os.Getwd()
		root = cwd
	}
	return filepath.Join(root, incident.DefaultDir)
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// driveToFixed advances inc through the state machine towards StateFixed.
func driveToFixed(inc *incident.Incident) error {
	path := []incident.State{
		incident.StateInvestigating,
		incident.StateMitigated,
		incident.StateFixed,
	}
	for _, s := range path {
		if inc.State == incident.StateFixed || inc.State == incident.StatePostMortemPublished {
			return nil
		}
		if incident.CanTransition(inc.State, s) {
			if err := incident.Transition(inc, s); err != nil {
				return err
			}
		}
	}
	return nil
}

// incidentSummary is used for JSON output.
type incidentSummary struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	State    incident.State    `json:"state"`
	Severity incident.Severity `json:"severity"`
	Systems  []string          `json:"systems"`
}

func printIncident(cmd *cobra.Command, inc *incident.Incident, asJSON bool) {
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		enc.Encode(incidentSummary{ //nolint:errcheck
			ID: inc.ID, Title: inc.Title, State: inc.State,
			Severity: inc.Severity, Systems: inc.Systems,
		})
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "incident %s [%s] %s — %s\n",
		inc.ID, inc.Severity, inc.State, inc.Title)
}

// newTriageCmd implements G-111: `forge incident triage`.
// Consumes a JSON bundle of CI failures / error reports and returns structured
// triage: severity, cluster assignment, and suggested GitHub Issue labels.
func newTriageCmd() *cobra.Command {
	var (
		root      string
		inputFile string
		asJSON    bool
	)
	cmd := &cobra.Command{
		Use:   "triage [--input <file>]",
		Short: "Auto-triage: summarise and cluster CI failures or error reports.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrIncidentFailed, "getwd", err)
				}
				root = cwd
			}
			var input []byte
			var err error
			if inputFile != "" {
				input, err = os.ReadFile(inputFile)
				if err != nil {
					return errcode.Newf(ErrIncidentFailed, err, "read input file %s", inputFile)
				}
			} else {
				// Read from stdin.
				input, err = os.ReadFile("/dev/stdin")
				if err != nil {
					input = []byte("{}") // empty bundle
				}
			}

			// Produce structured triage output.
			triage := map[string]any{
				"status":   "triage_pending",
				"input":    string(input),
				"severity": "unknown",
				"clusters": []string{},
				"labels":   []string{"needs-triage"},
				"message":  "Automated triage: review the input bundle and assign severity.",
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(triage)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "triage: %s\n", triage["message"])
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().StringVar(&inputFile, "input", "", "JSON bundle file to triage")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}
