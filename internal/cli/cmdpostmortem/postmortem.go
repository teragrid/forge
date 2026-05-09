// Package cmdpostmortem implements `forge postmortem` — lint and verify
// post-mortem documents stored in docs/postmortems/INC-*.md (ADR-020).
package cmdpostmortem

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// PostmortemDir is the project-relative directory that contains INC-*.md files.
const PostmortemDir = "docs/postmortems"

// Reserved error codes (range 3800–3899).
var (
	ErrPMFailed  = errcode.Register(errcode.Code(3800), "postmortem operation failed")
	ErrPMInvalid = errcode.Register(errcode.Code(3801), "postmortem document invalid")
	ErrPMDrift   = errcode.Register(errcode.Code(3802), "postmortem verification drift")
)

// The 8 mandatory section headings per ADR-020.
var requiredSections = []string{
	"## 1. Summary",
	"## 2. Impact",
	"## 3. Timeline",
	"## 4. Root cause",
	"## 5. What went well",
	"## 6. Action items",
	"## 7. Lessons / non-actions",
	"## 8. Bypass log",
}

// actionItemRE matches the canonical action-item shape from ADR-020:
//
//	- [ ] AI-NN — <description> — owner: @handle — due: YYYY-MM-DD — issue: #NNN
var actionItemRE = regexp.MustCompile(
	`^\s*-\s+\[\s*[xX ]?\s*\]\s+AI-\d+\s+—.*owner:\s+@\S+.*due:\s+\d{4}-\d{2}-\d{2}.*issue:\s+#\d+`)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "postmortem",
		Summary: "Lint and verify post-mortem documents (docs/postmortems/INC-*.md).",
		Inputs: []string{
			"[path]: file or directory to check (default: docs/postmortems/)",
			"--root <path> (default: cwd)",
			"--json",
		},
		Outputs:      []string{"stdout: per-file lint report"},
		SideEffects:  []string{"none (read-only)"},
		GatesTouched: []string{"§18.6 post-mortem CI gate (ADR-020)"},
		ErrorCodes:   []errcode.Code{ErrPMFailed, ErrPMInvalid, ErrPMDrift},
	})
}

// New returns the cobra command.
func New() *cobra.Command {
	var (
		root   string
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "postmortem [path]",
		Short: "Lint and verify post-mortem documents (ADR-020).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrPMFailed, "getwd", err)
				}
				root = cwd
			}
			dir := filepath.Join(root, PostmortemDir)
			if len(args) == 1 {
				dir = args[0]
			}
			reports, err := lintDir(dir)
			if err != nil {
				return errcode.New(ErrPMFailed, "lint dir", err)
			}
			return renderReports(cmd, reports, asJSON)
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

// FileReport is the lint result for one postmortem file.
type FileReport struct {
	File   string   `json:"file"`
	OK     bool     `json:"ok"`
	Issues []string `json:"issues,omitempty"`
}

func lintDir(dir string) ([]FileReport, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no postmortems yet — no error (absent is fine)
		}
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "INC-") && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	var reports []FileReport
	for _, f := range files {
		reports = append(reports, lintFile(f))
	}
	return reports, nil
}

func lintFile(path string) FileReport {
	r := FileReport{File: path}
	b, err := os.ReadFile(path)
	if err != nil {
		r.Issues = append(r.Issues, fmt.Sprintf("read error: %v", err))
		return r
	}
	content := string(b)
	// 1. All 8 sections present.
	for _, sec := range requiredSections {
		if !strings.Contains(content, sec) {
			r.Issues = append(r.Issues, fmt.Sprintf("missing section %q", sec))
		}
	}
	// 2. At least one action item in the correct shape.
	hasActionItem := false
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		if actionItemRE.MatchString(scanner.Text()) {
			hasActionItem = true
			break
		}
	}
	if !hasActionItem {
		r.Issues = append(r.Issues, "## 6. Action items: no valid action item found (need: - [ ] AI-NN — … — owner: @… — due: YYYY-MM-DD — issue: #NNN)")
	}
	// 3. At least one action item references a register (FR-NNN) or commit.
	if hasActionItem {
		hasRegisterRef := strings.Contains(content, "register: FR-") ||
			regexp.MustCompile(`commit:\s+[0-9a-f]{7,40}`).MatchString(content)
		if !hasRegisterRef {
			r.Issues = append(r.Issues, "## 6. Action items: ≥ 1 item must include 'register: FR-NNN' or 'commit: <sha>'")
		}
	}
	r.OK = len(r.Issues) == 0
	return r
}

func renderReports(cmd *cobra.Command, reports []FileReport, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(reports)
	}
	if len(reports) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "postmortem: no INC-*.md files found — nothing to check")
		return nil
	}
	var failed int
	for _, r := range reports {
		if r.OK {
			fmt.Fprintf(cmd.OutOrStdout(), "  PASS  %s\n", filepath.Base(r.File))
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  FAIL  %s\n", filepath.Base(r.File))
			for _, iss := range r.Issues {
				fmt.Fprintf(cmd.OutOrStdout(), "          - %s\n", iss)
			}
			failed++
		}
	}
	if failed > 0 {
		return errcode.Newf(ErrPMInvalid, nil, "%d of %d postmortem(s) failed lint", failed, len(reports))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "postmortem: %d file(s) OK ✓\n", len(reports))
	return nil
}
