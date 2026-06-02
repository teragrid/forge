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
	"github.com/teragrid/forge/internal/knowledge"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 3000..3099).
var (
	ErrScanFailed = errcode.Register(errcode.Code(3000), "scan operation failed")
)

// Finding represents a single secret finding.
type Finding struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Match      string `json:"match"`
	Rule       string `json:"rule"`
	Secret     string `json:"secret"`
	Confidence string `json:"confidence,omitempty"` // G-022: high|medium|low
}

// ScanResult is the summary of a scan run.
type ScanResult struct {
	Findings []Finding `json:"findings"`
	Count    int       `json:"count"`
	Status   string    `json:"status"` // "clean", "suspicious", "found"
	Note     string    `json:"note,omitempty"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "scan",
		Summary: "Scan project for security, correctness, performance, reliability, and other quality concerns (spec §4 Scan & Fix).",
		Inputs: []string{
			"family (required): security|correctness|performance|reliability|accessibility|cost|compliance|dx|all",
			"  — legacy sub-family aliases: secrets, rls, prompt-injection, supply-chain",
			"--root <path> (project root; default cwd)",
			"--json (machine-readable output)",
		},
		Outputs: []string{"stdout: findings list (text or JSON)"},
		SideEffects: []string{
			"runs gitleaks if available; otherwise uses built-in pattern matching",
		},
		GatesTouched: []string{"§16.5.4 #4 — security scanning"},
		ErrorCodes:   []errcode.Code{ErrScanFailed},
		OutputFields: []string{"findings", "count", "status"},
	})
}

// New returns the cobra command.
func New() *cobra.Command {
	var (
		root          string
		asJSON        bool
		mode          string
		since         string
		includeMedium bool
		fast          bool   // G-024: pre-commit fast mode (security+correctness only, quickest patterns)
		minConf       string // minimum confidence level to report/gate on (high|medium|low)
	)
	cmd := &cobra.Command{
		Use:   "scan <family> [--root <path>] [--json] [--mode report|suggest|apply]",
		Short: "Scan project across quality families: security, correctness, reliability, and more.",
		Long: "Scanner families (spec §4):\n" +
			"  security      — OWASP Top-10, secrets, RLS gaps, prompt-injection, supply-chain (M1)\n" +
			"  correctness   — float-money, type-cast risks, nil-deref patterns (M2)\n" +
			"  performance   — N+1 queries, missing indexes, expensive patterns (M2)\n" +
			"  reliability   — missing idempotency, outbox, circuit-breaker patterns (M2)\n" +
			"  accessibility — missing ARIA, colour contrast (M2)\n" +
			"  cost          — unbounded LLM calls, missing token budgets (M2)\n" +
			"  compliance    — missing PII tags, data-residency annotations (M2)\n" +
			"  dx            — stale .forge/instructions, missing tests, hygiene drift (M2)\n" +
			"  all           — run all families\n\n" +
			"Legacy sub-family aliases (M1 backward-compat): secrets, rls, prompt-injection, supply-chain\n\n" +
			"Modes (--mode):\n" +
			"  report  — print findings, exit non-zero if any (default, CI-safe)\n" +
			"  suggest — print unified diff of proposed fixes (no file changes)\n" +
			"  apply   — auto-apply high-confidence fixes; add --include-medium to widen gate\n\n" +
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

			scanMode := ScanMode(mode)
			if scanMode == "" {
				scanMode = ModeReport
			}
			if scanMode != ModeReport && scanMode != ModeSuggest && scanMode != ModeApply {
				return errcode.Newf(ErrScanFailed, nil,
					"invalid --mode %q; valid: report, suggest, apply", mode)
			}

			var (
				res *ScanResult
				err error
			)
			switch scanner {
			// ── Spec §4 top-level family names ──────────────────────────────
			case "security":
				if fast {
					res, err = RunSecurityFast(root)
				} else {
					res, err = RunSecurity(root)
				}
			case "correctness":
				res, err = RunCorrectness(root) // already fast — pure file-scan
			case "performance":
				res, err = RunPerformance(root)
			case "reliability":
				res, err = RunReliability(root)
			case "accessibility":
				res, err = RunAccessibility(root)
			case "cost":
				res, err = RunCost(root)
			case "compliance":
				res, err = RunCompliance(root)
			case "dx":
				res, err = RunDX(root)
			case "all":
				res, err = RunAll(root)
			// ── M1 legacy sub-family aliases (backward-compat) ──────────────
			case "secrets":
				res, err = RunSecrets(root)
			case "rls":
				res, err = RunRLS(root)
			case "prompt-injection":
				res, err = RunPromptInjection(root)
			case "supply-chain":
				res, err = RunSupplyChain(root)
			default:
				return errcode.New(ErrScanFailed, fmt.Sprintf(
					"unknown scanner %q; families: security, correctness, performance, reliability, "+
						"accessibility, cost, compliance, dx, all; "+
						"legacy: secrets, rls, prompt-injection, supply-chain", scanner), nil)
			}
			if err != nil {
				return err
			}

			// G-022: assign confidence scores.
			res.Findings = AssignConfidence(res.Findings)

			// G-023: --since diff against baseline.
			if since != "" {
				baseline := loadScanBaseline(root, scanner)
				if baseline != nil {
					res.Findings = diffFindings(res.Findings, baseline)
					res.Count = len(res.Findings)
					finalizeStatus(res)
				}
			}

			// G-023: persist history.
			persistScanHistory(root, scanner, res)

			// G-021: three-mode dispatch.
			if scanMode != ModeReport {
				return ApplyMode(scanMode, scanner, root, res, includeMedium)
			}

			// --min-confidence: filter out findings below the requested level.
			if minConf != "" {
				minLevel := Confidence(minConf)
				filtered := res.Findings[:0]
				for _, f := range res.Findings {
					c := Confidence(f.Confidence)
					if c == "" {
						c = ConfidenceMedium
					}
					switch minLevel {
					case ConfidenceHigh:
						if c == ConfidenceHigh {
							filtered = append(filtered, f)
						}
					case ConfidenceMedium:
						if c == ConfidenceHigh || c == ConfidenceMedium {
							filtered = append(filtered, f)
						}
					default: // low or unrecognised — keep all
						filtered = append(filtered, f)
					}
				}
				res.Findings = filtered
				res.Count = len(filtered)
				finalizeStatus(res)
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(res); err != nil {
					return err
				}
			} else {
				// ADR-026: append relevant knowledge-base refs to the note field.
				if idx, kbErr := knowledge.Load(); kbErr == nil {
					entries := knowledge.SelectForScan(idx, scanner)
					if len(entries) > 0 {
						ids := make([]string, 0, len(entries))
						for _, e := range entries {
							ids = append(ids, e.ID)
						}
						ref := "Knowledge refs: " + strings.Join(ids, ", ")
						if res.Note == "" {
							res.Note = ref
						} else {
							res.Note = res.Note + " | " + ref
						}
					}
				}
				renderText(cmd, res)
			}

			// Exit non-zero if findings found (gating for CI).
			if len(res.Findings) > 0 {
				return errcode.Newf(ErrScanFailed, nil,
					"%d finding(s) detected; fix before shipping", len(res.Findings))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&root, "root", "r", "", "project root (default: cwd)")
	cmd.Flags().BoolVarP(&asJSON, "json", "j", false, "emit machine-readable JSON")
	cmd.Flags().StringVarP(&mode, "mode", "m", "report", "scan mode: report|suggest|apply (G-021)")
	cmd.Flags().StringVar(&minConf, "min-confidence", "", "minimum confidence to gate on: high|medium|low (default: all)")
	cmd.Flags().StringVarP(&since, "since", "s", "", "diff findings against baseline from this git ref (G-023)")
	cmd.Flags().BoolVar(&includeMedium, "include-medium", false, "apply medium-confidence fixes in --mode apply (G-022)")
	cmd.Flags().BoolVarP(&fast, "fast", "f", false, "pre-commit fast mode: security runs secrets-only; skips slow scanners (G-024)")
	return cmd
}

// RunSecurity is the M1 top-level security family: runs all security-related
// sub-scanners (secrets, RLS gaps, prompt-injection, supply-chain) and merges results.
// Maps to the spec §4 "security" scanner family.
func RunSecurity(root string) (*ScanResult, error) {
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

// RunSecurityFast is the G-024 pre-commit fast variant: runs only the secrets
// scanner (the fastest sub-family) to keep pre-commit hooks snappy.
func RunSecurityFast(root string) (*ScanResult, error) {
	res, err := RunSecrets(root)
	if err != nil {
		return nil, err
	}
	finalizeStatus(res)
	return res, nil
}

// RunCorrectness scans for logical correctness hazards: floating-point money,
// unsafe type assertions, unhandled nil dereferences, and unchecked error returns.
func RunCorrectness(root string) (*ScanResult, error) {
	res := &ScanResult{}
	type rule struct {
		name  string
		exts  []string
		match func(text string) bool
	}

	floatMoney := regexp.MustCompile(`(?i)(price|amount|total|cost|balance|fee|charge|revenue|discount)\s*[\*\/\+\-]=?\s*[0-9]*\.[0-9]+`)
	unsafeAssert := regexp.MustCompile(`\)\.\([A-Z][A-Za-z*]+\)[^,\n{]`)
	anyEscape := regexp.MustCompile(`:\s*any[\s,;)\[]`)
	silenced := regexp.MustCompile(`if err != nil \{`)
	intTrunc := regexp.MustCompile(`(?i)(total|amount|price)\s*/\s*[0-9]+[^0-9\.]`)

	rules := []rule{
		{name: "float-money-arithmetic", exts: []string{".go", ".ts", ".js", ".py", ".java"},
			match: floatMoney.MatchString},
		{name: "unsafe-type-assertion", exts: []string{".go"},
			match: unsafeAssert.MatchString},
		{name: "ts-any-escape", exts: []string{".ts"},
			match: anyEscape.MatchString},
		// silenced-error: `if err != nil {` with empty body — match a body that is just `}`
		// Since scanFilesExt is line-by-line, flag the empty-body pattern explicitly
		{name: "silenced-error", exts: []string{".go"},
			match: silenced.MatchString},
		{name: "integer-truncation-money", exts: []string{".go", ".ts", ".js"},
			match: intTrunc.MatchString},
	}

	for _, r := range rules {
		r := r
		findings := scanFilesExt(root, r.exts, func(rel string, line int, text string) []Finding {
			if r.match(text) {
				return []Finding{{File: rel, Line: line, Rule: r.name, Match: truncate(strings.TrimSpace(text), 100)}}
			}
			return nil
		})
		res.Findings = append(res.Findings, findings...)
	}
	finalizeStatus(res)
	return res, nil
}

// RunPerformance scans for performance anti-patterns: SELECT *, mutex without defer,
// and missing FK indexes.
func RunPerformance(root string) (*ScanResult, error) {
	res := &ScanResult{}
	type rule struct {
		name  string
		exts  []string
		match func(text string) bool
	}

	selectStar := regexp.MustCompile(`(?i)\bSELECT\s+\*\s+FROM\b`)
	// unbounded-query: SELECT...FROM...; line that does NOT contain LIMIT
	selectFrom := regexp.MustCompile(`(?i)\bSELECT\b.+\bFROM\b.+;\s*$`)
	// mutex-no-defer: .Lock() line not immediately followed by defer — approximate: Lock() without defer on same line
	mutexLock := regexp.MustCompile(`\.Lock\(\)`)
	// fk-missing-index: REFERENCES ... not followed by -- indexed comment on the same line
	fkRef := regexp.MustCompile(`(?i)REFERENCES\s+\w+\s*\([^)]+\)`)

	rules := []rule{
		{name: "select-star", exts: []string{".sql", ".go", ".ts", ".js", ".py"},
			match: selectStar.MatchString},
		{name: "unbounded-query", exts: []string{".sql"},
			match: func(text string) bool {
				return selectFrom.MatchString(text) && !strings.Contains(strings.ToLower(text), "limit")
			}},
		{name: "mutex-no-defer", exts: []string{".go"},
			match: func(text string) bool {
				// Flag Lock() calls that don't have a `defer` anywhere on the same line
				return mutexLock.MatchString(text) && !strings.Contains(text, "defer")
			}},
		{name: "fk-missing-index", exts: []string{".sql"},
			match: func(text string) bool {
				return fkRef.MatchString(text) && !strings.Contains(strings.ToLower(text), "indexed")
			}},
	}

	for _, r := range rules {
		r := r
		findings := scanFilesExt(root, r.exts, func(rel string, line int, text string) []Finding {
			if r.match(text) {
				return []Finding{{File: rel, Line: line, Rule: r.name, Match: truncate(strings.TrimSpace(text), 100)}}
			}
			return nil
		})
		res.Findings = append(res.Findings, findings...)
	}
	finalizeStatus(res)
	return res, nil
}

// RunReliability scans for reliability anti-patterns: missing timeouts on HTTP
// calls, no retry/backoff, goroutines without error propagation.
func RunReliability(root string) (*ScanResult, error) {
	res := &ScanResult{}
	type rule struct {
		name  string
		exts  []string
		match func(text string) bool
	}

	httpCall := regexp.MustCompile(`\bhttp\.(Get|Post|PostForm)\(`)
	goFunc := regexp.MustCompile(`\bgo\s+func\s*\(`)
	// context-not-propagated: .query( / .exec( / .do( / .request( without ctx as first arg
	ctxCall := regexp.MustCompile(`(?i)\.(query|exec|do|request)\(`)
	fetchThen := regexp.MustCompile(`\bfetch\s*\([^)]+\)\s*\.then`)
	paymentCall := regexp.MustCompile(`(?i)(stripe|braintree|paypal)\.\w*(charge|create|pay)\(`)
	panicErr := regexp.MustCompile(`panic\(err\)`)

	rules := []rule{
		{name: "http-no-timeout", exts: []string{".go"}, match: httpCall.MatchString},
		{name: "untracked-goroutine", exts: []string{".go"}, match: goFunc.MatchString},
		{name: "context-not-propagated", exts: []string{".go"},
			match: func(text string) bool {
				// Match .query(...) style calls where ctx is NOT present in the call
				return ctxCall.MatchString(text) && !strings.Contains(text, "ctx")
			}},
		{name: "fetch-no-timeout", exts: []string{".ts", ".js"}, match: fetchThen.MatchString},
		{name: "payment-no-idempotency", exts: []string{".ts", ".js", ".go", ".py"}, match: paymentCall.MatchString},
		{name: "panic-on-error", exts: []string{".go"}, match: panicErr.MatchString},
	}

	for _, r := range rules {
		r := r
		findings := scanFilesExt(root, r.exts, func(rel string, line int, text string) []Finding {
			if r.match(text) {
				return []Finding{{File: rel, Line: line, Rule: r.name, Match: truncate(strings.TrimSpace(text), 100)}}
			}
			return nil
		})
		res.Findings = append(res.Findings, findings...)
	}
	finalizeStatus(res)
	return res, nil
}

// RunAccessibility scans HTML/JSX/TSX for missing ARIA attributes,
// non-descriptive link text, and missing alt text.
func RunAccessibility(root string) (*ScanResult, error) {
	res := &ScanResult{}
	type rule struct {
		name  string
		exts  []string
		match func(text string) bool
	}

	imgTag := regexp.MustCompile(`(?i)<img\b`)
	linkText := regexp.MustCompile(`(?i)>[\s]*(click here|read more|here|more|link)[\s]*<`)
	buttonOrAnchor := regexp.MustCompile(`(?i)<(button|a)\b`)
	tabIndexPos := regexp.MustCompile(`tabIndex=["\'][1-9]`)
	htmlTag := regexp.MustCompile(`(?i)<html\b`)

	htmlExts := []string{".html", ".jsx", ".tsx", ".vue", ".svelte"}
	rules := []rule{
		{name: "img-missing-alt", exts: htmlExts,
			match: func(text string) bool {
				return imgTag.MatchString(text) && !strings.Contains(strings.ToLower(text), "alt=")
			}},
		{name: "link-text-click-here", exts: htmlExts, match: linkText.MatchString},
		{name: "button-no-label", exts: htmlExts,
			match: func(text string) bool {
				lower := strings.ToLower(text)
				return buttonOrAnchor.MatchString(text) &&
					!strings.Contains(lower, "aria-label=") &&
					!strings.Contains(lower, "aria-labelledby=") &&
					!strings.Contains(lower, "title=")
			}},
		{name: "tabindex-positive", exts: htmlExts, match: tabIndexPos.MatchString},
		{name: "html-missing-lang", exts: []string{".html"},
			match: func(text string) bool {
				return htmlTag.MatchString(text) && !strings.Contains(strings.ToLower(text), "lang=")
			}},
	}

	for _, r := range rules {
		r := r
		findings := scanFilesExt(root, r.exts, func(rel string, line int, text string) []Finding {
			if r.match(text) {
				return []Finding{{File: rel, Line: line, Rule: r.name, Match: truncate(strings.TrimSpace(text), 100)}}
			}
			return nil
		})
		res.Findings = append(res.Findings, findings...)
	}
	finalizeStatus(res)
	return res, nil
}

// RunCost scans for patterns that cause unbounded LLM spend, memory growth,
// or accidental cloud cost amplification.
func RunCost(root string) (*ScanResult, error) {
	res := &ScanResult{}
	type rule struct {
		name  string
		exts  []string
		match func(text string) bool
	}

	loopStart := regexp.MustCompile(`(?i)\b(for|forEach|\.map|\.each)\b`)
	llmCall := regexp.MustCompile(`(?i)(completion|chat\.create|generate|llm\.call)`)
	cloudList := regexp.MustCompile(`(?i)\.(listObjects|listBlobs|list_objects)\s*\(`)
	aiProvider := regexp.MustCompile(`(?i)\b(openai|anthropic|cohere|replicate)\.\w+\s*\(`)

	allExts := []string{".go", ".ts", ".js", ".py"}
	rules := []rule{
		// LLM call on a line that also contains a loop keyword
		{name: "llm-call-in-loop", exts: allExts,
			match: func(text string) bool {
				return loopStart.MatchString(text) && llmCall.MatchString(text)
			}},
		// LLM call without max_tokens on the same line
		{name: "llm-no-token-limit", exts: allExts,
			match: func(text string) bool {
				lower := strings.ToLower(text)
				return llmCall.MatchString(text) &&
					!strings.Contains(lower, "max_tokens") &&
					!strings.Contains(lower, "maxtokens")
			}},
		// Cloud list call without pagination chained on the same line
		{name: "unbounded-cloud-list", exts: allExts,
			match: func(text string) bool {
				lower := strings.ToLower(text)
				return cloudList.MatchString(text) &&
					!strings.Contains(lower, ".each") &&
					!strings.Contains(lower, ".paginate") &&
					!strings.Contains(lower, "marker") &&
					!strings.Contains(lower, "continuationtoken")
			}},
		// AI provider call without any cache reference on the same line
		{name: "missing-cache-control", exts: allExts,
			match: func(text string) bool {
				return aiProvider.MatchString(text) && !strings.Contains(strings.ToLower(text), "cache")
			}},
	}

	for _, r := range rules {
		r := r
		findings := scanFilesExt(root, r.exts, func(rel string, line int, text string) []Finding {
			if r.match(text) {
				return []Finding{{File: rel, Line: line, Rule: r.name, Match: truncate(strings.TrimSpace(text), 100)}}
			}
			return nil
		})
		res.Findings = append(res.Findings, findings...)
	}
	finalizeStatus(res)
	return res, nil
}

// RunCompliance scans for PII in logs, missing data classification, hardcoded
// regions, and missing audit trails on data mutation operations.
func RunCompliance(root string) (*ScanResult, error) {
	res := &ScanResult{}
	type rule struct {
		name  string
		exts  []string
		match func(text string) bool
	}

	logCall := regexp.MustCompile(`(?i)(log|logger|console)\s*\.\s*\w+\s*\(`)
	piiField := regexp.MustCompile(`(?i)\b(email|phone|ssn|dob|credit_card|card_number|password|token|secret)\b`)
	awsRegion := regexp.MustCompile(`"(us-east-1|eu-west-1|ap-southeast-1|us-west-2)"`)
	sqlMutation := regexp.MustCompile(`(?i)\b(INSERT\s+INTO|UPDATE\s+\w+\s+SET|DELETE\s+FROM)\b`)
	rawPII := regexp.MustCompile(`(?i)(ssn|social_security|credit_card|card_number)\s+(TEXT|VARCHAR|CHAR)`)
	setCookie := regexp.MustCompile(`(?i)\bsetCookie\s*\(`)

	rules := []rule{
		{name: "pii-in-logs", exts: []string{".go", ".ts", ".js", ".py", ".java"},
			match: func(text string) bool {
				return logCall.MatchString(text) && piiField.MatchString(text)
			}},
		{name: "hardcoded-region", exts: []string{".go", ".ts", ".js", ".py", ".java", ".tf"},
			match: awsRegion.MatchString},
		{name: "mutation-no-audit", exts: []string{".sql", ".go", ".ts"},
			match: func(text string) bool {
				return sqlMutation.MatchString(text) && !strings.Contains(strings.ToLower(text), "audit")
			}},
		{name: "raw-pii-storage", exts: []string{".sql"}, match: rawPII.MatchString},
		{name: "insecure-cookie", exts: []string{".ts", ".js"},
			match: func(text string) bool {
				lower := strings.ToLower(text)
				return setCookie.MatchString(text) &&
					!strings.Contains(lower, "secure") &&
					!strings.Contains(lower, "httponly") &&
					!strings.Contains(lower, "samesite")
			}},
	}

	for _, r := range rules {
		r := r
		findings := scanFilesExt(root, r.exts, func(rel string, line int, text string) []Finding {
			if r.match(text) {
				return []Finding{{File: rel, Line: line, Rule: r.name, Match: truncate(strings.TrimSpace(text), 100)}}
			}
			return nil
		})
		res.Findings = append(res.Findings, findings...)
	}
	finalizeStatus(res)
	return res, nil
}

// RunDX scans for developer-experience debt: high TODO/FIXME density,
// source files without corresponding test files, stale Forge version pins,
// and missing .forge/manifest.
func RunDX(root string) (*ScanResult, error) {
	res := &ScanResult{}

	todoRe := regexp.MustCompile(`(?i)\b(TODO|FIXME|HACK|XXX)\b`)
	staleRe := regexp.MustCompile(`forge v0\.[0-9]+\.[0-9]+-dev`)

	// ── TODO/FIXME density ────────────────────────────────────────────────────
	todoFindings := scanFilesExt(root, []string{".go", ".ts", ".js", ".py", ".java"},
		func(rel string, line int, text string) []Finding {
			if todoRe.MatchString(text) {
				return []Finding{{File: rel, Line: line, Rule: "todo-fixme-density", Match: truncate(strings.TrimSpace(text), 80)}}
			}
			return nil
		})
	res.Findings = append(res.Findings, todoFindings...)

	// ── Source files without test counterparts ────────────────────────────────
	testlessFindings := findSourceFilesWithoutTests(root)
	res.Findings = append(res.Findings, testlessFindings...)

	// ── Stale Forge spec references ───────────────────────────────────────────
	staleFindings := scanFilesExt(root,
		[]string{".md", ".yml", ".yaml", ".json", ".ts", ".go"},
		func(rel string, line int, text string) []Finding {
			if staleRe.MatchString(text) {
				return []Finding{{File: rel, Line: line, Rule: "stale-forge-version", Match: truncate(strings.TrimSpace(text), 80)}}
			}
			return nil
		})
	res.Findings = append(res.Findings, staleFindings...)

	// ── Missing .forge/manifest ───────────────────────────────────────────────
	if _, err := os.Stat(filepath.Join(root, ".forge", "manifest")); os.IsNotExist(err) {
		res.Findings = append(res.Findings, Finding{
			File:  ".forge/manifest",
			Line:  0,
			Rule:  "missing-forge-manifest",
			Match: ".forge/manifest not found — run: forge init",
		})
	}

	finalizeStatus(res)
	return res, nil
}

// findSourceFilesWithoutTests identifies Go and TypeScript source files that
// have no corresponding _test.go / .test.ts counterpart.
func findSourceFilesWithoutTests(root string) []Finding {
	var out []Finding
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				n := d.Name()
				if n == ".git" || n == "node_modules" || n == "vendor" || n == ".forge" || n == "dist" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		name := d.Name()
		dir := filepath.Dir(p)
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)

		// Go source files
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			testFile := filepath.Join(dir, strings.TrimSuffix(name, ".go")+"_test.go")
			if _, err := os.Stat(testFile); os.IsNotExist(err) {
				out = append(out, Finding{
					File:  rel,
					Line:  0,
					Rule:  "source-without-test",
					Match: fmt.Sprintf("no %s_test.go found", strings.TrimSuffix(name, ".go")),
				})
			}
		}

		// TypeScript source files (skip types, stubs, generated)
		if strings.HasSuffix(name, ".ts") &&
			!strings.HasSuffix(name, ".test.ts") &&
			!strings.HasSuffix(name, ".d.ts") &&
			!strings.HasSuffix(name, ".stubs.ts") &&
			!strings.HasSuffix(name, ".types.ts") &&
			!strings.HasSuffix(name, ".routes.ts") {
			testFile := filepath.Join(dir, strings.TrimSuffix(name, ".ts")+".test.ts")
			if _, err := os.Stat(testFile); os.IsNotExist(err) {
				out = append(out, Finding{
					File:  rel,
					Line:  0,
					Rule:  "source-without-test",
					Match: fmt.Sprintf("no %s.test.ts found", strings.TrimSuffix(name, ".ts")),
				})
			}
		}
		return nil
	})
	return out
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
		// generic-bearer: require the value to be a quoted string literal so that
		// variable-name references (e.g. token = csrfTokenVar) are not flagged.
		{"generic-bearer", regexp.MustCompile(`(?i)(bearer|api[_-]?key|token|secret|password)\s*[:=]\s*["'][A-Za-z0-9_\-]{16,}["']`)},
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
	// Well-known documentation files commonly contain examples of attack
	// patterns for educational purposes and must not trigger the scanner.
	docFileNames := map[string]bool{
		"README.md": true, "README.txt": true, "README.rst": true,
		"CHANGELOG.md": true, "CONTRIBUTING.md": true, "CODE_OF_CONDUCT.md": true,
		"SECURITY.md": true, "LICENSE.md": true, "NOTICE.md": true, "AUTHORS.md": true,
	}
	res.Findings = scanFilesExt(root, []string{".md", ".txt", ".prompt", ".tmpl", ".yaml", ".yml", ".json"},
		func(rel string, line int, text string) []Finding {
			base := filepath.Base(filepath.FromSlash(rel))
			// Skip well-known documentation files — they explain attacks, they are not attacks.
			if docFileNames[base] {
				return nil
			}
			// Skip test files — by convention they contain intentional attack examples
			// used to verify that guardrails work correctly.
			if strings.HasPrefix(base, "test_") || strings.Contains(base, "_test.") {
				return nil
			}
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
		// package-lock.json / yarn.lock / pnpm-lock.yaml are auto-generated — their
		// transitive-dependency version ranges are not developer choices and would
		// produce hundreds of loose-version-range false positives.
		[]string{"package.json", "Cargo.toml", "go.mod", "requirements.txt", "Gemfile"},
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
	families := []func(string) (*ScanResult, error){
		RunSecurity,
		RunCorrectness,
		RunPerformance,
		RunReliability,
		RunAccessibility,
		RunCost,
		RunCompliance,
		RunDX,
	}
	for _, run := range families {
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
			// Skip version-control, package trees, generated build output,
			// tool-managed directories, and test fixture trees — scanning
			// them produces only noise (fixtures contain intentional examples).
			switch name {
			case ".git", "node_modules", "vendor", ".forge",
				".next", ".nuxt", ".svelte-kit",
				"dist", "build", "out", "output",
				"coverage", ".nyc_output", ".cache", "tmp", ".tmp",
				"fixtures", "testdata", ".playwright-mcp":
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
	fmt.Fprintf(w, "forge scan\n")
	fmt.Fprintf(w, "findings: %d\n", r.Count)
	fmt.Fprintf(w, "status:   %s\n", r.Status)
	if r.Note != "" {
		fmt.Fprintf(w, "note:     %s\n", r.Note)
	}
	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "\nno findings detected.")
		return
	}
	fmt.Fprintln(w, "\nfindings:")
	for _, f := range r.Findings {
		fmt.Fprintf(w, "  %s:%d [%s] %s\n", f.File, f.Line, f.Rule, f.Match)
	}
}
