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
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check local environment health.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, _ := os.Getwd()
			rep := Run(root)
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
