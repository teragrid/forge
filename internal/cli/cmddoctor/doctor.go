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
// Package cmddoctor implements `forge doctor` (DEV-M0-14).
package cmddoctor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 1200..1299).
var (
	ErrEnvUnhealthy = errcode.Register(errcode.Code(1200), "environment health check failed")
	ErrDoctorDrift  = errcode.Register(errcode.Code(1201), ".gitignore managed block drift detected")
)

// Status enumerates per-check outcomes.
type Status string

const (
	StatusOK   Status = "ok"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// Check is a single environment check result.
type Check struct {
	Name     string `json:"name"`
	Status   Status `json:"status"`
	Detail   string `json:"detail"`
	Required bool   `json:"required"`
	Hint     string `json:"hint,omitempty"`
}

// Report is the full doctor output.
type Report struct {
	OS      string  `json:"os"`
	Arch    string  `json:"arch"`
	Checks  []Check `json:"checks"`
	Healthy bool    `json:"healthy"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:        "doctor",
		Summary:     "Check the local environment for forge prerequisites.",
		Inputs:      []string{"--json (machine-readable output)"},
		Outputs:     []string{"stdout: per-check status (text or JSON)", "exit: 0 ok, non-zero on any required-check failure"},
		SideEffects: []string{"creates a temp file in os.TempDir() to verify writability"},
		ErrorCodes:  []errcode.Code{ErrEnvUnhealthy},
	})
}

// New returns the cobra command.
func New() *cobra.Command {
	var (
		asJSON bool
		drift  bool
	)
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check local environment health.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, _ := os.Getwd()
			rep := Run(root)
			// G-114: --drift appends schema-drift checks.
			if drift {
				rep.Checks = append(rep.Checks, checkSchemaDrift(root)...)
				rep.Healthy = true
				for _, c := range rep.Checks {
					if c.Required && c.Status == StatusFail {
						rep.Healthy = false
					}
				}
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(rep); err != nil {
					return err
				}
			} else {
				renderText(cmd, rep)
			}
			if !rep.Healthy {
				return errcode.New(ErrEnvUnhealthy,
					"one or more required checks failed; see output above", nil)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	cmd.Flags().BoolVar(&drift, "drift", false, "also check for schema drift (G-114)")
	return cmd
}

// Run executes the checks and returns a Report. root is the project root
// directory; pass an empty string to auto-detect from the working directory.
// Exposed for tests and future callers that want to embed the doctor output.
func Run(root string) Report {
	if root == "" {
		root, _ = os.Getwd()
	}
	rep := Report{OS: runtime.GOOS, Arch: runtime.GOARCH}
	rep.Checks = append(rep.Checks,
		checkBinary("git", true, "install git from https://git-scm.com/downloads"),
		checkBinary("go", true, "install Go 1.24+ from https://go.dev/dl/"),
		checkTempWritable(),
		checkGitignoreDrift(root),
		checkLLMModeAdvisory(),
	)
	rep.Healthy = true
	for _, c := range rep.Checks {
		if c.Required && c.Status == StatusFail {
			rep.Healthy = false
		}
	}
	return rep
}

func checkBinary(name string, required bool, hint string) Check {
	path, err := exec.LookPath(name)
	if err != nil {
		return Check{Name: name + " on PATH", Status: StatusFail, Required: required,
			Detail: "not found", Hint: hint}
	}
	return Check{Name: name + " on PATH", Status: StatusOK, Required: required,
		Detail: path}
}

func checkTempWritable() Check {
	dir := os.TempDir()
	f, err := os.CreateTemp(dir, "forge-doctor-*")
	if err != nil {
		return Check{Name: "temp dir writable", Status: StatusFail, Required: true,
			Detail: dir, Hint: "ensure $TMPDIR / %TEMP% points to a writable location"}
	}
	_ = f.Close()
	_ = os.Remove(f.Name())
	return Check{Name: "temp dir writable", Status: StatusOK, Required: true,
		Detail: filepath.ToSlash(dir)}
}

// ── .gitignore drift check ────────────────────────────────────────────────

// giDriftStart / giDriftEnd are the same markers used by the "gitignore"
// codemod (internal/codemod/hygiene.go). We redeclare them here to avoid an
// import cycle (cmddoctor → codemod is fine, but codemod imports nothing from
// cmddoctor so there is no cycle — we could import directly; using local
// constants keeps the dependency graph clean).
const (
	giDriftStart = "# forge:gitignore:start"
	giDriftEnd   = "# forge:gitignore:end"
)

// canonicalGiSnippet is used only for content comparison. It must stay in
// sync with codemod.canonicalGitignoreBlock. The drift check compares the
// entire managed block (start + body + end) found in the file against this.
const canonicalGiSnippet = `# forge:gitignore:start
# Managed by forge — do not edit this block. Run "forge upgrade gitignore" to refresh.
# forge-version: managed
.forge/scratch/
.forge/cache/
*.tmp
*.bak
__pycache__/
# forge:gitignore:end
`

// checkGitignoreDrift reports whether the .gitignore managed block has
// drifted from the canonical content known to this version of forge.
func checkGitignoreDrift(root string) Check {
	const checkName = ".gitignore managed block"
	path := filepath.Join(root, ".gitignore")
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Check{Name: checkName, Status: StatusOK, Required: false,
				Detail: "no .gitignore present"}
		}
		return Check{Name: checkName, Status: StatusWarn, Required: false,
			Detail: "cannot read .gitignore: " + err.Error()}
	}
	content := string(body)
	si := strings.Index(content, giDriftStart)
	ei := strings.Index(content, giDriftEnd)
	if si < 0 || ei <= si {
		// No managed block — that is fine; user has not run `forge upgrade gitignore` yet.
		return Check{Name: checkName, Status: StatusOK, Required: false,
			Detail: "no managed block present"}
	}
	// Extract the managed block (inclusive of end marker + trailing newline).
	endIdx := ei + len(giDriftEnd)
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}
	block := content[si:endIdx]
	if block == canonicalGiSnippet {
		return Check{Name: checkName, Status: StatusOK, Required: false,
			Detail: "up to date"}
	}
	return Check{Name: checkName, Status: StatusWarn, Required: false,
		Detail: "managed block differs from current canonical content",
		Hint:   "run 'forge upgrade gitignore --apply' to refresh"}
}

// checkSchemaDrift detects schema drift by comparing generated schema files
// against their source definitions. Returns one Check per schema category.
// G-114: opens a PR suggestion if drift is detected (stub for non-CI envs).
func checkSchemaDrift(root string) []Check {
	type schemaEntry struct {
		label   string
		genPath string // generated file path relative to root
		srcPath string // canonical source file relative to root
	}
	entries := []schemaEntry{
		{
			label:   "clischema generated",
			genPath: "internal/clischema/schema_gen.go",
			srcPath: "internal/clischema/schema.go",
		},
		{
			label:   "error codes doc",
			genPath: "docs/ERROR_CODES.md",
			srcPath: "cmd/gen-errors/main.go",
		},
	}
	var checks []Check
	for _, e := range entries {
		genFull := filepath.Join(root, e.genPath)
		srcFull := filepath.Join(root, e.srcPath)
		genInfo, genErr := os.Stat(genFull)
		srcInfo, srcErr := os.Stat(srcFull)
		if genErr != nil && os.IsNotExist(genErr) {
			checks = append(checks, Check{
				Name:   e.label + " drift",
				Status: StatusWarn, Required: false,
				Detail: "generated file not present: " + e.genPath,
				Hint:   "run the generator to create it",
			})
			continue
		}
		if srcErr != nil {
			checks = append(checks, Check{
				Name:   e.label + " drift",
				Status: StatusWarn, Required: false,
				Detail: "source file not found: " + e.srcPath,
			})
			continue
		}
		// Drift heuristic: if source is newer than generated, flag a warning.
		if genErr == nil && srcErr == nil && srcInfo.ModTime().After(genInfo.ModTime()) {
			checks = append(checks, Check{
				Name:   e.label + " drift",
				Status: StatusWarn, Required: false,
				Detail: fmt.Sprintf("%s is newer than %s — regenerate", e.srcPath, e.genPath),
				Hint:   "run the appropriate generator; or open a PR via 'forge doctor --drift --pr'",
			})
		} else {
			checks = append(checks, Check{
				Name:   e.label + " drift",
				Status: StatusOK, Required: false,
				Detail: "up to date",
			})
		}
	}
	return checks
}

func renderText(cmd *cobra.Command, r Report) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "forge doctor — %s/%s\n", r.OS, r.Arch)
	for _, c := range r.Checks {
		marker := "[OK]  "
		if c.Status == StatusWarn {
			marker = "[WARN]"
		} else if c.Status == StatusFail {
			marker = "[FAIL]"
		}
		fmt.Fprintf(w, "  %s %-22s %s\n", marker, c.Name, c.Detail)
		if c.Hint != "" && c.Status != StatusOK {
			fmt.Fprintf(w, "         hint: %s\n", c.Hint)
		}
	}
	if r.Healthy {
		fmt.Fprintln(w, "\nall required checks passed.")
	} else {
		fmt.Fprintln(w, "\nat least one required check failed.")
	}
}

// checkLLMModeAdvisory advises on the FORGE_LLM_MODE environment variable.
// It is informational (not required) — just surfaces the capability so LLMs
// and operators know how to opt in or out.
func checkLLMModeAdvisory() Check {
	val := os.Getenv("FORGE_LLM_MODE")
	switch val {
	case "1":
		return Check{
			Name:     "llm-mode",
			Status:   StatusOK,
			Required: false,
			Detail:   "FORGE_LLM_MODE=1 — JSON envelopes + gate suppression active",
		}
	case "":
		return Check{
			Name:     "llm-mode",
			Status:   StatusWarn,
			Required: false,
			Detail:   "FORGE_LLM_MODE not set — human-readable output by default",
			Hint:     "set FORGE_LLM_MODE=1 when an LLM is driving forge commands (enables JSON envelopes + suppresses y/N prompts)",
		}
	default:
		return Check{
			Name:     "llm-mode",
			Status:   StatusWarn,
			Required: false,
			Detail:   fmt.Sprintf("FORGE_LLM_MODE=%q — expected '1' or unset", val),
			Hint:     "set FORGE_LLM_MODE=1 to enable LLM mode, or unset to use human mode",
		}
	}
}
