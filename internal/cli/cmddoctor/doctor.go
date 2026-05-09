// Package cmddoctor implements `forge doctor` (DEV-M0-14).
package cmddoctor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 1200..1299).
var (
	ErrEnvUnhealthy = errcode.Register(errcode.Code(1200), "environment health check failed")
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
			rep := Run()
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

// Run executes the checks and returns a Report. Exposed for tests + future
// callers that want to embed the doctor output.
func Run() Report {
	rep := Report{OS: runtime.GOOS, Arch: runtime.GOARCH}
	rep.Checks = append(rep.Checks,
		checkBinary("git", true, "install git from https://git-scm.com/downloads"),
		checkBinary("go", true, "install Go 1.24+ from https://go.dev/dl/"),
		checkTempWritable(),
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
