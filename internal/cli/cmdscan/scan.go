// Package cmdscan implements `forge scan` (M1 headline + M2 expansion).
// Scanner families: secrets (M1), rls/prompt-injection/supply-chain (M2).
package cmdscan

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 3000..3099).
var (
	ErrScanFailed = errcode.Register(errcode.Code(3000), "scan operation failed")
)

// Finding represents a single secret finding.
type Finding struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Match  string `json:"match"`
	Rule   string `json:"rule"`
	Secret string `json:"secret"`
}

// ScanResult is the summary of a scan run.
type ScanResult struct {
	Findings []Finding `json:"findings"`
	Count    int       `json:"count"`
	Status   string    `json:"status"` // "clean", "suspicious", "found"
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "scan",
		Summary: "Scan project for secrets, RLS leaks, prompt-injection, supply-chain risks (M1 security loop).",
		Inputs: []string{
			"secrets (required; scan for API keys, passwords, tokens)",
			"--root <path> (project root; default cwd)",
			"--json (machine-readable output)",
		},
		Outputs: []string{"stdout: findings list (text or JSON)"},
		SideEffects: []string{
			"runs gitleaks if available; otherwise uses built-in pattern matching",
		},
		GatesTouched: []string{"§16.5.4 #4 — security scanning"},
		ErrorCodes:   []errcode.Code{ErrScanFailed},
	})
}

// New returns the cobra command.
func New() *cobra.Command {
	var (
		root   string
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "scan <family> [--root <path>] [--json]",
		Short: "Scan project for secrets, RLS leaks, prompt-injection, or supply-chain risks.",
		Long: "Scanner families: secrets, rls, prompt-injection, supply-chain. " +
			"Integrates with gitleaks for secrets if available; built-in patterns otherwise. " +
			"Exits non-zero if findings detected (CI-gateable).",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scanner := args[0]
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrScanFailed, "getwd", err)
				}
				root = cwd
			}

			var (
				res *ScanResult
				err error
			)
			switch scanner {
			case "secrets":
				res, err = RunSecrets(root)
			case "rls":
				res, err = RunRLS(root)
			case "prompt-injection":
				res, err = RunPromptInjection(root)
			case "supply-chain":
				res, err = RunSupplyChain(root)
			case "all":
				res, err = RunAll(root)
			default:
				return errcode.New(ErrScanFailed, fmt.Sprintf(
					"unknown scanner %q; one of: secrets, rls, prompt-injection, supply-chain, all", scanner), nil)
			}
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(res); err != nil {
					return err
				}
			} else {
				renderText(cmd, res)
			}

			// Exit non-zero if findings found (gating for CI).
			if len(res.Findings) > 0 {
				return errcode.Newf(ErrScanFailed, nil,
					"%d secret finding(s) detected; fix before shipping", len(res.Findings))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

// RunSecrets scans for secrets. Returns findings and status.
func RunSecrets(root string) (*ScanResult, error) {
	res := &ScanResult{}

	// Try gitleaks first if available.
	if hasGitleaks() {
		findings, err := scanWithGitleaks(root)
		if err != nil && !strings.Contains(err.Error(), "executable file not found") {
			return nil, errcode.New(ErrScanFailed, "gitleaks", err)
		}
		res.Findings = findings
	} else {
		// Fallback to built-in pattern matching.
		res.Findings = scanWithBuiltinPatterns(root)
	}

	finalizeStatus(res)
	return res, nil
}

func hasGitleaks() bool {
	_, err := exec.LookPath("gitleaks")
	return err == nil
}

func scanWithGitleaks(root string) ([]Finding, error) {
	cmd := exec.Command("gitleaks", "detect", "--source", root, "--report-format", "json", "--no-color")
	out, err := cmd.Output()
	if err != nil && cmd.ProcessState.ExitCode() != 1 {
		return nil, err
	}
	// gitleaks returns exit code 1 if findings found (non-zero); we just process the output.
	if len(out) == 0 {
		return []Finding{}, nil
	}
	var findings []Finding
	type gitleaksMatch struct {
		File   string `json:"File"`
		Line   int    `json:"StartLine"`
		Match  string `json:"Match"`
		Secret string `json:"Secret"`
	}
	var matches []gitleaksMatch
	_ = json.Unmarshal(out, &matches)
	for _, m := range matches {
		findings = append(findings, Finding{
			File:   m.File,
			Line:   m.Line,
			Match:  m.Match,
			Rule:   "gitleaks",
			Secret: m.Secret,
		})
	}
	return findings, nil
}

func scanWithBuiltinPatterns(root string) []Finding {
	rules := []struct {
		Name    string
		Pattern *regexp.Regexp
	}{
		{"aws-access-key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
		{"openai-key", regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`)},
		{"github-token", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{30,}`)},
		{"private-key-block", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
		{"generic-bearer", regexp.MustCompile(`(?i)(bearer|api[_-]?key|token|secret|password)\s*[:=]\s*["']?[A-Za-z0-9_\-]{16,}["']?`)},
	}
	return scanFiles(root, func(rel string, line int, text string) []Finding {
		var out []Finding
		for _, r := range rules {
			if loc := r.Pattern.FindStringIndex(text); loc != nil {
				out = append(out, Finding{
					File: rel, Line: line, Rule: r.Name,
					Match: truncate(text, 80), Secret: text[loc[0]:loc[1]],
				})
			}
		}
		return out
	})
}

// RunRLS scans for missing Row-Level-Security in SQL/migration files.
func RunRLS(root string) (*ScanResult, error) {
	res := &ScanResult{}
	rules := []struct {
		Name    string
		Pattern *regexp.Regexp
	}{
		// CREATE TABLE without subsequent ENABLE ROW LEVEL SECURITY is a signal,
		// but we keep it line-local: flag any CREATE TABLE missing tenant column.
		{"missing-tenant-col-create-table", regexp.MustCompile(`(?i)create\s+table\s+\w+\s*\(`)},
		{"select-without-where-tenant", regexp.MustCompile(`(?i)select\s+.+\s+from\s+\w+\s*;`)},
	}
	res.Findings = scanFilesExt(root, []string{".sql", ".pgsql"}, func(rel string, line int, text string) []Finding {
		var out []Finding
		for _, r := range rules {
			if r.Pattern.MatchString(text) &&
				!strings.Contains(strings.ToLower(text), "tenant") &&
				!strings.Contains(strings.ToLower(text), "workspace") {
				out = append(out, Finding{
					File: rel, Line: line, Rule: r.Name, Match: truncate(text, 100), Secret: "",
				})
			}
		}
		return out
	})
	finalizeStatus(res)
	return res, nil
}

// RunPromptInjection scans prompt templates / instructions for known
// injection vectors (ignore-previous-instructions, role-impersonation, etc.).
func RunPromptInjection(root string) (*ScanResult, error) {
	res := &ScanResult{}
	rules := []struct {
		Name    string
		Pattern *regexp.Regexp
	}{
		{"ignore-previous", regexp.MustCompile(`(?i)ignore\s+(all\s+)?previous\s+instructions`)},
		{"role-override", regexp.MustCompile(`(?i)you\s+are\s+now\s+(a\s+)?(?:dan|jailbroken|unrestricted)`)},
		{"system-prompt-leak", regexp.MustCompile(`(?i)(reveal|print|show|dump)\s+(your|the)\s+system\s+prompt`)},
		{"unsafe-eval", regexp.MustCompile(`(?i)execute\s+(this\s+)?(arbitrary\s+)?code`)},
	}
	res.Findings = scanFilesExt(root, []string{".md", ".txt", ".prompt", ".tmpl", ".yaml", ".yml", ".json"},
		func(rel string, line int, text string) []Finding {
			var out []Finding
			for _, r := range rules {
				if r.Pattern.MatchString(text) {
					out = append(out, Finding{
						File: rel, Line: line, Rule: r.Name, Match: truncate(text, 100), Secret: "",
					})
				}
			}
			return out
		})
	finalizeStatus(res)
	return res, nil
}

// RunSupplyChain checks dependency files for risky pinning patterns
// (no version pin, wildcard, github-tarball without commit, etc.).
func RunSupplyChain(root string) (*ScanResult, error) {
	res := &ScanResult{}
	risky := map[*regexp.Regexp]string{
		regexp.MustCompile(`"\^|"~|"\*"`):                         "loose-version-range",
		regexp.MustCompile(`"git\+https?://[^#]+"`):               "unpinned-git-url",
		regexp.MustCompile(`(?i)curl\s+[^|]+\|\s*(sh|bash)`):      "curl-pipe-shell",
		regexp.MustCompile(`(?i)(replace|exclude)\s+\S+\s+=>\s+`): "go-mod-replace-directive",
	}
	res.Findings = scanFilesExt(root,
		[]string{"package.json", "package-lock.json", "Cargo.toml", "go.mod", "requirements.txt", "Gemfile"},
		func(rel string, line int, text string) []Finding {
			var out []Finding
			for re, name := range risky {
				if re.MatchString(text) {
					out = append(out, Finding{
						File: rel, Line: line, Rule: name, Match: truncate(text, 100), Secret: "",
					})
				}
			}
			return out
		})
	finalizeStatus(res)
	return res, nil
}

// RunAll runs every scanner family and merges results.
func RunAll(root string) (*ScanResult, error) {
	merged := &ScanResult{}
	for _, run := range []func(string) (*ScanResult, error){
		RunSecrets, RunRLS, RunPromptInjection, RunSupplyChain,
	} {
		r, err := run(root)
		if err != nil {
			return nil, err
		}
		merged.Findings = append(merged.Findings, r.Findings...)
	}
	finalizeStatus(merged)
	return merged, nil
}

// scanFiles walks root and applies fn to every line of every text-ish file.
func scanFiles(root string, fn func(rel string, line int, text string) []Finding) []Finding {
	return scanFilesExt(root, nil, fn)
}

// scanFilesExt is like scanFiles but restricts to files matching one of exts
// (suffix match). Empty/nil exts means all text files.
func scanFilesExt(root string, exts []string, fn func(rel string, line int, text string) []Finding) []Finding {
	var out []Finding
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == ".forge" {
				return filepath.SkipDir
			}
			return nil
		}
		if !matchesExts(d.Name(), exts) {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		ln := 0
		for sc.Scan() {
			ln++
			out = append(out, fn(rel, ln, sc.Text())...)
		}
		return nil
	})
	return out
}

func matchesExts(name string, exts []string) bool {
	if len(exts) == 0 {
		// Default: skip obvious binaries.
		for _, b := range []string{".exe", ".so", ".a", ".dll", ".dylib", ".png", ".jpg", ".jpeg", ".gif", ".pdf", ".zip", ".tar", ".gz"} {
			if strings.HasSuffix(name, b) {
				return false
			}
		}
		return true
	}
	for _, e := range exts {
		if strings.HasSuffix(name, e) || name == e {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func finalizeStatus(r *ScanResult) {
	r.Count = len(r.Findings)
	switch {
	case r.Count == 0:
		r.Status = "clean"
	case r.Count < 5:
		r.Status = "suspicious"
	default:
		r.Status = "found"
	}
}

func renderText(cmd *cobra.Command, r *ScanResult) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "forge scan secrets\n")
	fmt.Fprintf(w, "findings: %d\n", r.Count)
	fmt.Fprintf(w, "status:   %s\n", r.Status)
	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "\nno secrets detected.")
		return
	}
	fmt.Fprintln(w, "\nfindings:")
	for _, f := range r.Findings {
		fmt.Fprintf(w, "  %s:%d [%s] %s\n", f.File, f.Line, f.Rule, f.Match)
	}
}
