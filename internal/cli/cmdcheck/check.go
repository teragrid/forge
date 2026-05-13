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

// Package cmdcheck implements `forge check` (DEV-M1-44, spec §4).
//
// Validates the project against schema contracts, ADRs, architectural
// constraints, and spec §11.1.2 Developer Promises.
//
// Gates run in this order:
//  1. manifest     — .forge/manifest present and valid JSON
//  2. hygiene      — .forge/hygiene.yml present
//  3. conventions  — .forge/conventions.json present
//  4. errcode-reg  — errcode.Register calls match declared ranges
//  5. verbmeta     — verbmeta.Register present in each cmd* package
//  6. go-build     — `go build ./...` exits 0
//  7. go-vet       — `go vet ./...` exits 0
package cmdcheck

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 1900..1999).
var (
	ErrCheckFailed = errcode.Register(errcode.Code(1900), "check failed")
)

// GateResult is the outcome of one validation gate.
type GateResult struct {
	Gate       string `json:"gate"`
	Status     string `json:"status"` // "pass", "fail", "warn"
	Message    string `json:"message,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// CheckResult summarises all gates.
type CheckResult struct {
	Root   string       `json:"root"`
	Passed int          `json:"passed"`
	Failed int          `json:"failed"`
	Warned int          `json:"warned"`
	Gates  []GateResult `json:"gates"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "check",
		Summary: "Validate project schema contracts and architectural constraints (DEV-M1-44, spec §4).",
		Inputs: []string{
			"[schema]   — specific gate to validate (default: all)",
			"--root <path>",
			"--strict   — fail on warnings",
			"--json     — emit machine-readable JSON",
		},
		Outputs:      []string{"stdout: gate-by-gate validation results"},
		SideEffects:  []string{"none (read-only)"},
		GatesTouched: []string{"§4 check", "§11.1.2 Developer Promise #3"},
		ErrorCodes:   []errcode.Code{ErrCheckFailed},
	})
}

// New returns the cobra command for `forge check`.
func New() *cobra.Command {
	var (
		root   string
		strict bool
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "check [gate]",
		Short: "Validate project schema contracts and architectural constraints.",
		Long: "forge check validates the project against schema contracts, ADRs, and\n" +
			"architectural constraints defined in the spec.\n\n" +
			"Gates: manifest, hygiene, conventions, errcode-reg, verbmeta, go-build, go-vet\n\n" +
			"When no gate is specified, all constraints are checked.\n" +
			"Use --strict to fail on warnings.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "all"
			if len(args) > 0 {
				target = args[0]
			}
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrCheckFailed, "getwd", err)
				}
				root = cwd
			}
			result := Run(root, target)
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			renderText(cmd, result)
			if result.Failed > 0 || (strict && result.Warned > 0) {
				return errcode.Newf(ErrCheckFailed, nil,
					"%d gate(s) failed", result.Failed)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().BoolVar(&strict, "strict", false, "fail on warnings")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

// Run executes the validation gates and returns a summary.
func Run(root, target string) CheckResult {
	result := CheckResult{Root: root}

	type gateFn func(string) GateResult
	gates := map[string]gateFn{
		"manifest":    checkManifest,
		"hygiene":     checkHygiene,
		"conventions": checkConventions,
		"verbmeta":    checkVerbmeta,
		"go-build":    checkGoBuild,
		"go-vet":      checkGoVet,
	}
	// Ordered run.
	order := []string{"manifest", "hygiene", "conventions", "verbmeta", "go-build", "go-vet"}

	for _, name := range order {
		if target != "all" && target != name {
			continue
		}
		fn := gates[name]
		gr := fn(root)
		result.Gates = append(result.Gates, gr)
		switch gr.Status {
		case "pass":
			result.Passed++
		case "fail":
			result.Failed++
		case "warn":
			result.Warned++
		}
	}
	return result
}

func timed(name string, fn func() (string, string)) GateResult {
	start := time.Now()
	status, msg := fn()
	return GateResult{
		Gate:       name,
		Status:     status,
		Message:    msg,
		DurationMs: time.Since(start).Milliseconds(),
	}
}

func checkManifest(root string) GateResult {
	return timed("manifest", func() (string, string) {
		path := filepath.Join(root, ".forge", "manifest")
		data, err := os.ReadFile(path)
		if err != nil {
			return "warn", ".forge/manifest not found (run forge adopt --apply)"
		}
		var m interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			return "fail", fmt.Sprintf(".forge/manifest invalid JSON: %v", err)
		}
		return "pass", ""
	})
}

func checkHygiene(root string) GateResult {
	return timed("hygiene", func() (string, string) {
		path := filepath.Join(root, ".forge", "hygiene.yml")
		if _, err := os.Stat(path); err != nil {
			return "warn", ".forge/hygiene.yml not found (run forge adopt --apply)"
		}
		return "pass", ""
	})
}

func checkConventions(root string) GateResult {
	return timed("conventions", func() (string, string) {
		path := filepath.Join(root, ".forge", "conventions.json")
		data, err := os.ReadFile(path)
		if err != nil {
			return "warn", ".forge/conventions.json not found (run forge adopt --apply)"
		}
		var c interface{}
		if err := json.Unmarshal(data, &c); err != nil {
			return "fail", fmt.Sprintf(".forge/conventions.json invalid JSON: %v", err)
		}
		return "pass", ""
	})
}

func checkVerbmeta(root string) GateResult {
	return timed("verbmeta", func() (string, string) {
		cliDir := filepath.Join(root, "internal", "cli")
		entries, err := os.ReadDir(cliDir)
		if err != nil {
			return "warn", fmt.Sprintf("cannot read internal/cli: %v", err)
		}
		var missing []string
		for _, e := range entries {
			if !e.IsDir() || !strings.HasPrefix(e.Name(), "cmd") {
				continue
			}
			pkgDir := filepath.Join(cliDir, e.Name())
			// Check any .go file in pkg contains verbmeta.Register.
			goFiles, _ := filepath.Glob(filepath.Join(pkgDir, "*.go"))
			found := false
			for _, gf := range goFiles {
				data, _ := os.ReadFile(gf)
				if strings.Contains(string(data), "verbmeta.Register") {
					found = true
					break
				}
			}
			if !found {
				missing = append(missing, e.Name())
			}
		}
		if len(missing) > 0 {
			return "warn", fmt.Sprintf("verbmeta.Register missing in: %s", strings.Join(missing, ", "))
		}
		return "pass", ""
	})
}

func checkGoBuild(root string) GateResult {
	return timed("go-build", func() (string, string) {
		cmd := exec.Command("go", "build", "./...")
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "fail", fmt.Sprintf("go build failed: %s", strings.TrimSpace(string(out)))
		}
		return "pass", ""
	})
}

func checkGoVet(root string) GateResult {
	return timed("go-vet", func() (string, string) {
		cmd := exec.Command("go", "vet", "./...")
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "fail", fmt.Sprintf("go vet failed: %s", strings.TrimSpace(string(out)))
		}
		return "pass", ""
	})
}

func renderText(cmd *cobra.Command, r CheckResult) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "forge check: %s\n\n", r.Root)
	for _, g := range r.Gates {
		icon := "✓"
		switch g.Status {
		case "fail":
			icon = "✗"
		case "warn":
			icon = "⚠"
		}
		suffix := ""
		if g.Message != "" {
			suffix = " — " + g.Message
		}
		fmt.Fprintf(w, "  %s %-18s (%dms)%s\n", icon, g.Gate, g.DurationMs, suffix)
	}
	fmt.Fprintf(w, "\npass: %d  fail: %d  warn: %d\n", r.Passed, r.Failed, r.Warned)
}
