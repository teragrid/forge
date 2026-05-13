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
package cmdaudit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/failure"
)

// Reserved error codes for failure-register sub-commands (range 3700–3799).
var (
	ErrFRFailed  = errcode.Register(errcode.Code(3700), "failure-register operation failed")
	ErrFRInvalid = errcode.Register(errcode.Code(3701), "failure-register schema invalid")
	ErrFRDrift   = errcode.Register(errcode.Code(3702), "failure-register parity drift detected")
)

// newFailureRegisterCmd returns the `forge audit failure-register` subcommand
// with three sub-subcommands: verify, list, lint.
func newFailureRegisterCmd() *cobra.Command {
	var (
		root   string
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "failure-register <verify|list|lint>",
		Short: "Manage the failure-register (ADR-016).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrFRFailed, "getwd", err)
				}
				root = cwd
			}
			path := filepath.Join(root, failure.DefaultPath)
			reg, err := failure.Load(path)
			if err != nil {
				return errcode.New(ErrFRFailed, "load failure-register", err)
			}
			switch args[0] {
			case "verify":
				return frVerify(cmd, reg, asJSON)
			case "list":
				return frList(cmd, reg, asJSON)
			case "lint":
				return frLint(cmd, reg, asJSON)
			default:
				return errcode.Newf(ErrFRFailed, nil, "unknown subcommand %q (verify|list|lint)", args[0])
			}
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

// frLint validates the register schema only (fast, no parity checks).
func frLint(cmd *cobra.Command, reg *failure.Register, asJSON bool) error {
	err := reg.Validate()
	type result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if asJSON {
		r := result{OK: err == nil}
		if err != nil {
			r.Error = err.Error()
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "failure-register lint: INVALID — %v\n", err)
		return errcode.New(ErrFRInvalid, "schema lint", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "failure-register lint: schema OK ✓")
	return nil
}

// frList dumps active (non-retired) entries.
func frList(cmd *cobra.Command, reg *failure.Register, asJSON bool) error {
	if err := reg.Validate(); err != nil {
		return errcode.New(ErrFRInvalid, "validate before list", err)
	}
	active := reg.Active()
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(active)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "failure-register: %d active entries\n", len(active))
	for _, e := range active {
		sev := string(e.SeverityDefault)
		if sev == "" {
			sev = "?"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %-8s %-20s [%s] %s\n", e.ID, e.Component, sev, e.FailureMode)
	}
	return nil
}

// frVerify runs schema validation and detects entries that lack a test anchor,
// which is the minimum "parity" check before a full Arch §17.2 diff is wired.
func frVerify(cmd *cobra.Command, reg *failure.Register, asJSON bool) error {
	if err := reg.Validate(); err != nil {
		return errcode.New(ErrFRInvalid, "schema validation", err)
	}
	type drift struct {
		ID     string `json:"id"`
		Reason string `json:"reason"`
	}
	var drifts []drift
	for _, e := range reg.Active() {
		if e.TestAnchor == "" {
			drifts = append(drifts, drift{ID: e.ID, Reason: "missing test_anchor"})
		}
	}
	type report struct {
		OK     bool    `json:"ok"`
		Drifts []drift `json:"drifts,omitempty"`
	}
	r := report{OK: len(drifts) == 0, Drifts: drifts}
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
	if r.OK {
		fmt.Fprintf(cmd.OutOrStdout(), "failure-register verify: %d entries, no drift ✓\n", len(reg.Active()))
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "failure-register verify: %d drift(s) found\n", len(drifts))
	for _, d := range drifts {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s — %s\n", d.ID, d.Reason)
	}
	return errcode.Newf(ErrFRDrift, nil, "%d drift(s) in failure-register", len(drifts))
}
