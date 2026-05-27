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

// hook.go — quality-gate hook system for forge ship checkpoints.
//
// Hooks are lightweight quality gates that run before or after each checkpoint.
// They are inspired by the SA Power "6 automated quality gates" pattern from
// the forge knowledge base.
//
// Hook phases:
//
//	PhasePreCheckpoint  — runs before the checkpoint LLM call; a Passed=false
//	                      result adds a warning prefix to the checkpoint detail.
//	PhasePostCheckpoint — runs after the checkpoint completes; a Passed=false
//	                      result flags the checkpoint as "warning" (non-blocking
//	                      by default; set HookConfig.Strict to elevate to "fail").
//	PhasePostPipeline   — runs once after all checkpoints pass; used for
//	                      learning extraction, review routing, and KB updates.
//
// Default hooks balanced across all 7 forge ship checkpoints:
//
//	self-review-gate          — pre-checkpoint (all): detect placeholders/hedging before writing
//	spec-completeness-gate    — post-checkpoint (spec): AC section present + Given/When/Then
//	adr-quality-gate          — post-checkpoint (arch): require 2+ alternatives + rationale
//	arch-file-lint            — post-checkpoint (arch): detect empty sections / TODO markers
//	tdd-gate                  — post-checkpoint (test): no always-passing assertions
//	breakdown-completeness    — post-checkpoint (breakdown): all tasks have done criteria
//	task-completion-gate      — post-checkpoint (code): all tasks marked done in tasks.md
//	security-hygiene-gate     — post-checkpoint (code): secret + sandbox path scan
//	qa-coverage-gate          — post-checkpoint (qa-verify): AC items referenced in tests
package cmdship

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HookPhase enumerates when a hook fires relative to a checkpoint.
type HookPhase string

const (
	// PhasePreCheckpoint fires before the checkpoint function executes.
	PhasePreCheckpoint HookPhase = "pre-checkpoint"
	// PhasePostCheckpoint fires after the checkpoint function returns.
	PhasePostCheckpoint HookPhase = "post-checkpoint"
	// PhasePostPipeline fires once after all checkpoints pass (res.Ready == true).
	PhasePostPipeline HookPhase = "post-pipeline"
)

// HookContext carries the runtime context available to a hook handler.
type HookContext struct {
	// Phase is the current hook phase.
	Phase HookPhase
	// CheckpointName is the lower-cased checkpoint name (e.g. "spec", "arch").
	CheckpointName string
	// Root is the project root directory.
	Root string
	// Description is the feature description.
	Description string
	// Pipe is the active LLM pipe (may be nil in dry-run / no-LLM mode).
	Pipe *LLMPipe
	// Result is the checkpoint result; nil during PhasePreCheckpoint.
	Result *Checkpoint
}

// HookResult is what a hook handler returns.
type HookResult struct {
	// Passed is false when the hook detected an issue.
	Passed bool
	// Message is the human-readable explanation (empty when Passed=true).
	Message string
}

// Hook is a single quality gate attached to the pipeline.
type Hook struct {
	// Name uniquely identifies this hook (used in --skip-hook and logs).
	Name string
	// Phase controls when this hook fires.
	Phase HookPhase
	// Gate restricts the hook to a specific checkpoint name (lower-cased).
	// An empty Gate means "run for all checkpoints in this phase".
	Gate string
	// Handler is the function that performs the quality check.
	Handler func(ctx HookContext) HookResult
}

// HookConfig controls runtime behaviour of the hook system.
// Loaded from .forge/hooks.yaml when present; defaults used otherwise.
type HookConfig struct {
	// Disabled is the list of hook names to skip.
	Disabled []string
	// Strict causes post-checkpoint hook failures to escalate from "warning"
	// to "fail" (stops the pipeline). Default: false.
	Strict bool
}

// loadHookConfig reads .forge/hooks.yaml if it exists.
// Missing file → default config (no disabled hooks, not strict).
func loadHookConfig(root string) HookConfig {
	path := filepath.Join(root, ".forge", "hooks.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return HookConfig{}
	}
	// Minimal YAML parsing: only support "disabled:" and "strict:" lines.
	cfg := HookConfig{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "strict: true") {
			cfg.Strict = true
		}
		if strings.HasPrefix(line, "- ") {
			cfg.Disabled = append(cfg.Disabled, strings.TrimPrefix(line, "- "))
		}
	}
	return cfg
}

// isHookDisabled reports whether hookName appears in cfg.Disabled.
func isHookDisabled(cfg HookConfig, hookName string) bool {
	for _, d := range cfg.Disabled {
		if d == hookName {
			return true
		}
	}
	return false
}

// ── Default hooks ─────────────────────────────────────────────────────────────

// selfReviewGate checks for placeholder text and hedging language in the
// artefacts produced at any checkpoint (pre-write check).
// Maps to the SA Power "Self-Review Gate" (preToolUse hook).
var selfReviewGate = Hook{
	Name:  "self-review-gate",
	Phase: PhasePreCheckpoint,
	Gate:  "", // applies to ALL checkpoints
	Handler: func(ctx HookContext) HookResult {
		// Map each checkpoint to the artefact files it produces.
		slug := slugify(ctx.Description)
		base := filepath.Join(ctx.Root, ".forge", "specs", slug)
		artifactsByCheckpoint := map[string][]string{
			"spec":      {filepath.Join(base, "spec.md")},
			"arch":      {filepath.Join(base, "arch.md"), filepath.Join(base, "adr.md")},
			"test":      {filepath.Join(base, "tests.md")},
			"breakdown": {filepath.Join(base, "tasks.md")},
			"code":      {filepath.Join(base, "impl-notes.md")},
			"ship":      {filepath.Join(base, "ship-checklist.md")},
			"qa-verify": {filepath.Join(base, "qa-report.md")},
		}
		filesToScan, ok := artifactsByCheckpoint[ctx.CheckpointName]
		if !ok {
			return HookResult{Passed: true}
		}

		badPatterns := []string{"TODO", "TBD", "<fill", "...", "might consider", "could perhaps"}
		for _, fp := range filesToScan {
			data, err := os.ReadFile(fp)
			if err != nil {
				continue // file not yet written — nothing to scan
			}
			content := string(data)
			for _, pat := range badPatterns {
				if strings.Contains(content, pat) {
					return HookResult{
						Passed:  false,
						Message: fmt.Sprintf("self-review-gate: placeholder/hedging in %s: %q", filepath.Base(fp), pat),
					}
				}
			}
		}
		return HookResult{Passed: true}
	},
}

// specCompletenessGate verifies the spec checkpoint produced an AC section
// with at least one Given/When/Then acceptance criterion.
var specCompletenessGate = Hook{
	Name:  "spec-completeness-gate",
	Phase: PhasePostCheckpoint,
	Gate:  "spec",
	Handler: func(ctx HookContext) HookResult {
		if ctx.Result == nil || ctx.Result.Status == "fail" {
			return HookResult{Passed: true}
		}
		slug := slugify(ctx.Description)
		specPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "spec.md")
		data, err := os.ReadFile(specPath)
		if err != nil {
			return HookResult{Passed: true} // no spec file yet
		}
		content := string(data)
		// Must have an Acceptance Criteria / AC section.
		hasACSection := strings.Contains(strings.ToLower(content), "acceptance criteria") ||
			strings.Contains(strings.ToLower(content), "## ac")
		if !hasACSection {
			return HookResult{
				Passed:  false,
				Message: "spec-completeness-gate: spec.md missing Acceptance Criteria section",
			}
		}
		// At least one Given/When/Then criterion.
		hasGWT := strings.Contains(content, "Given ") &&
			strings.Contains(content, "When ") &&
			strings.Contains(content, "Then ")
		if !hasGWT {
			return HookResult{
				Passed:  false,
				Message: "spec-completeness-gate: spec.md AC missing Given/When/Then format",
			}
		}
		return HookResult{Passed: true}
	},
}

// taskCompletionGate checks that every targeted task in tasks.md is marked done.
var taskCompletionGate = Hook{
	Name:  "task-completion-gate",
	Phase: PhasePostCheckpoint,
	Gate:  "code",
	Handler: func(ctx HookContext) HookResult {
		if ctx.Result == nil || ctx.Result.Status == "fail" {
			return HookResult{Passed: true} // nothing to check on failure
		}
		slug := slugify(ctx.Description)
		tasksPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "tasks.md")
		data, err := os.ReadFile(tasksPath)
		if err != nil {
			return HookResult{Passed: true} // no tasks file → pass
		}
		lines := strings.Split(string(data), "\n")
		incomplete := 0
		for _, l := range lines {
			if strings.Contains(l, "- [ ]") {
				incomplete++
			}
		}
		if incomplete > 0 {
			return HookResult{
				Passed:  false,
				Message: fmt.Sprintf("task-completion-gate: %d incomplete task(s) remain in tasks.md after code checkpoint", incomplete),
			}
		}
		return HookResult{Passed: true}
	},
}

// adrQualityGate verifies that the ADR produced by the arch checkpoint contains
// at least 2 alternatives and an explicit consequences section.
var adrQualityGate = Hook{
	Name:  "adr-quality-gate",
	Phase: PhasePostCheckpoint,
	Gate:  "arch",
	Handler: func(ctx HookContext) HookResult {
		if ctx.Result == nil || ctx.Result.Status == "fail" {
			return HookResult{Passed: true}
		}
		slug := slugify(ctx.Description)
		adrPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "adr.md")
		data, err := os.ReadFile(adrPath)
		if err != nil {
			return HookResult{Passed: true} // no ADR file → nothing to check
		}
		content := strings.ToLower(string(data))
		// Look for at least 2 alternative headings or list items.
		altCount := strings.Count(content, "alternative") + strings.Count(content, "option ")
		if altCount < 2 {
			return HookResult{
				Passed:  false,
				Message: "adr-quality-gate: ADR must evaluate ≥2 alternatives; found fewer markers",
			}
		}
		if !strings.Contains(content, "consequence") && !strings.Contains(content, "trade-off") {
			return HookResult{
				Passed:  false,
				Message: "adr-quality-gate: ADR missing consequences/trade-offs section",
			}
		}
		return HookResult{Passed: true}
	},
}

// archFileLint scans architecture artefacts for empty sections and TODO markers.
var archFileLint = Hook{
	Name:  "arch-file-lint",
	Phase: PhasePostCheckpoint,
	Gate:  "arch",
	Handler: func(ctx HookContext) HookResult {
		if ctx.Result == nil || ctx.Result.Status == "fail" {
			return HookResult{Passed: true}
		}
		slug := slugify(ctx.Description)
		archPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "arch.md")
		data, err := os.ReadFile(archPath)
		if err != nil {
			return HookResult{Passed: true}
		}
		// Detect consecutive heading followed immediately by another heading
		// (empty section) or a TODO marker.
		lines := strings.Split(string(data), "\n")
		for i, l := range lines {
			trimmed := strings.TrimSpace(l)
			if strings.HasPrefix(trimmed, "#") && i+1 < len(lines) {
				next := strings.TrimSpace(lines[i+1])
				if strings.HasPrefix(next, "#") {
					return HookResult{
						Passed:  false,
						Message: fmt.Sprintf("arch-file-lint: empty section detected after %q", trimmed),
					}
				}
			}
			if strings.Contains(trimmed, "TODO") || strings.Contains(trimmed, "TBD") {
				return HookResult{
					Passed:  false,
					Message: fmt.Sprintf("arch-file-lint: placeholder detected in arch.md: %q", trimmed),
				}
			}
		}
		return HookResult{Passed: true}
	},
}

// tddGate checks that test files contain at least one assertion and do not
// contain obviously always-passing patterns (assert(true), empty test bodies).
// Maps to the SA Power "Task Completion Gate" applied at the test checkpoint.
var tddGate = Hook{
	Name:  "tdd-gate",
	Phase: PhasePostCheckpoint,
	Gate:  "test",
	Handler: func(ctx HookContext) HookResult {
		if ctx.Result == nil || ctx.Result.Status == "fail" {
			return HookResult{Passed: true}
		}
		slug := slugify(ctx.Description)
		testsPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "tests.md")
		data, err := os.ReadFile(testsPath)
		if err != nil {
			return HookResult{Passed: true} // no test artefact yet
		}
		content := string(data)
		// Detect always-passing anti-patterns.
		alwaysPass := []string{
			"assert(true)", "assertTrue(true)", "expect(true).toBe(true)",
			"assert.True(t, true)", "t.Skip(", "// TODO: test",
		}
		for _, pat := range alwaysPass {
			if strings.Contains(content, pat) {
				return HookResult{
					Passed:  false,
					Message: fmt.Sprintf("tdd-gate: always-passing or skipped test pattern detected: %q", pat),
				}
			}
		}
		// Must reference at least one Given/When/Then or test scenario.
		if !strings.Contains(content, "Given ") && !strings.Contains(content, "Scenario:") &&
			!strings.Contains(content, "func Test") {
			return HookResult{
				Passed:  false,
				Message: "tdd-gate: tests.md must contain at least one test scenario (Given/When/Then or func Test*)",
			}
		}
		return HookResult{Passed: true}
	},
}

// breakdownCompletenessGate verifies every task in tasks.md has done criteria
// (not just a checkbox — the line must have content after the marker).
// Maps to the SA Power "Task Completion Gate" applied at the breakdown checkpoint.
var breakdownCompletenessGate = Hook{
	Name:  "breakdown-completeness",
	Phase: PhasePostCheckpoint,
	Gate:  "breakdown",
	Handler: func(ctx HookContext) HookResult {
		if ctx.Result == nil || ctx.Result.Status == "fail" {
			return HookResult{Passed: true}
		}
		slug := slugify(ctx.Description)
		tasksPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "tasks.md")
		data, err := os.ReadFile(tasksPath)
		if err != nil {
			return HookResult{Passed: true}
		}
		lines := strings.Split(string(data), "\n")
		empty := 0
		for _, l := range lines {
			trimmed := strings.TrimSpace(l)
			// A task line with no description after the checkbox marker.
			if trimmed == "- [ ]" || trimmed == "- [x]" || trimmed == "- [X]" {
				empty++
			}
		}
		if empty > 0 {
			return HookResult{
				Passed:  false,
				Message: fmt.Sprintf("breakdown-completeness: %d task(s) have empty descriptions in tasks.md", empty),
			}
		}
		return HookResult{Passed: true}
	},
}

// securityHygieneGate scans code artefact notes for common secret patterns and
// sandbox-bypass indicators. Maps to the SA Power "Auto-Review Gate" for code.
var securityHygieneGate = Hook{
	Name:  "security-hygiene-gate",
	Phase: PhasePostCheckpoint,
	Gate:  "code",
	Handler: func(ctx HookContext) HookResult {
		if ctx.Result == nil || ctx.Result.Status == "fail" {
			return HookResult{Passed: true}
		}
		slug := slugify(ctx.Description)
		implPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "impl-notes.md")
		data, err := os.ReadFile(implPath)
		if err != nil {
			return HookResult{Passed: true}
		}
		content := string(data)
		// Secret-like patterns.
		secretPatterns := []string{
			"password =", "api_key =", "apikey =", "secret =",
			"token =", "private_key", "AWS_SECRET", "-----BEGIN",
		}
		for _, pat := range secretPatterns {
			if strings.Contains(strings.ToLower(content), strings.ToLower(pat)) {
				return HookResult{
					Passed:  false,
					Message: fmt.Sprintf("security-hygiene-gate: potential secret pattern %q in impl-notes.md", pat),
				}
			}
		}
		// Shell injection indicators.
		shellPatterns := []string{"shell=true", "os.system(", "exec.Command(\"sh\",", "exec.Command(\"bash\","}
		for _, pat := range shellPatterns {
			if strings.Contains(content, pat) {
				return HookResult{
					Passed:  false,
					Message: fmt.Sprintf("security-hygiene-gate: shell-injection risk: %q found in impl-notes.md", pat),
				}
			}
		}
		return HookResult{Passed: true}
	},
}

// qaCoverageGate checks that the qa-verify artefact references each acceptance
// criterion from the spec (by scanning for "AC" or "Given" markers).
var qaCoverageGate = Hook{
	Name:  "qa-coverage-gate",
	Phase: PhasePostCheckpoint,
	Gate:  "qa-verify",
	Handler: func(ctx HookContext) HookResult {
		if ctx.Result == nil || ctx.Result.Status == "fail" {
			return HookResult{Passed: true}
		}
		slug := slugify(ctx.Description)
		specPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "spec.md")
		qaPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "qa-report.md")

		specData, specErr := os.ReadFile(specPath)
		qaData, qaErr := os.ReadFile(qaPath)
		if specErr != nil || qaErr != nil {
			return HookResult{Passed: true} // artefacts not present yet
		}

		// Count AC items in spec (lines starting with "Given" or "- AC-").
		specContent := string(specData)
		qaContent := string(qaData)

		acCount := 0
		covered := 0
		for _, line := range strings.Split(specContent, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "Given ") || strings.HasPrefix(trimmed, "- AC-") {
				acCount++
				// Check if qa-report references the same line (first 30 chars as key).
				key := trimmed
				if len(key) > 30 {
					key = key[:30]
				}
				if strings.Contains(qaContent, key) {
					covered++
				}
			}
		}
		if acCount > 0 && covered < acCount {
			return HookResult{
				Passed:  false,
				Message: fmt.Sprintf("qa-coverage-gate: %d/%d AC items referenced in qa-report.md", covered, acCount),
			}
		}
		return HookResult{Passed: true}
	},
}

// specCodeAlignmentHandler is the shared logic for specCodeAlignmentGateCode
// and specCodeAlignmentGateQA — wraps auditSpecVsCode() and fails the gate when
// blocking gaps are found (incomplete tasks, untested authz roles, missing event tests).
var specCodeAlignmentHandler = func(ctx HookContext) HookResult {
	if ctx.Result == nil || ctx.Result.Status == "fail" {
		return HookResult{Passed: true}
	}
	result := auditSpecVsCode(ctx.Root, ctx.Description)
	if !result.HasBlockingGaps() {
		return HookResult{Passed: true}
	}
	var msgs []string
	for _, g := range result.Gaps {
		if g.Severity == "blocking" {
			msgs = append(msgs, fmt.Sprintf("[%s] %s (hint: %s)", g.Type, g.Description, g.Hint))
		}
	}
	return HookResult{
		Passed:  false,
		Message: fmt.Sprintf("spec-code-alignment-gate: %d blocking gap(s): %s", len(msgs), strings.Join(msgs, "; ")),
	}
}

// specCodeAlignmentGateCode runs the spec-vs-code audit at the code checkpoint.
// Ensures all tasks are done, authz roles tested, and event assertions present
// before the code checkpoint is considered complete.
var specCodeAlignmentGateCode = Hook{
	Name:    "spec-code-alignment-gate",
	Phase:   PhasePostCheckpoint,
	Gate:    "code",
	Handler: specCodeAlignmentHandler,
}

// specCodeAlignmentGateQA runs the same spec-vs-code audit as the FIRST phase
// of qa-verify — deterministic, zero tokens, BLOCKING. If blocking gaps exist
// the qa-verify checkpoint fails immediately before any LLM call is made.
var specCodeAlignmentGateQA = Hook{
	Name:    "spec-code-alignment-gate",
	Phase:   PhasePostCheckpoint,
	Gate:    "qa-verify",
	Handler: specCodeAlignmentHandler,
}

// manualTestPlanGate verifies that generateManualTestPlan() wrote a complete
// manual-test-plan.md for the current feature, and that all 6 role sections
// are present. A missing or incomplete plan means qa-verify is not complete.
var manualTestPlanGate = Hook{
	Name:  "manual-test-plan-gate",
	Phase: PhasePostCheckpoint,
	Gate:  "qa-verify",
	Handler: func(ctx HookContext) HookResult {
		if ctx.Result == nil || ctx.Result.Status == "fail" {
			return HookResult{Passed: true}
		}
		slug := slugify(ctx.Description)
		planPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "manual-test-plan.md")
		data, err := os.ReadFile(planPath)
		if err != nil {
			return HookResult{
				Passed:  false,
				Message: "manual-test-plan-gate: manual-test-plan.md not found — run qa-verify with an LLM configured",
			}
		}
		content := strings.ToLower(string(data))
		// Each of the 6 role sections must be identifiable by a heading keyword.
		requiredSections := []struct {
			keyword string
			role    string
		}{
			{"product owner", "Product Owner (UAT)"},
			{"business analyst", "Business Analyst"},
			{"quality engineer", "Quality Engineer"},
			{"security", "Security Reviewer"},
			{"devops", "DevOps / SRE"},
			{"compliance", "Compliance / CPO"},
		}
		var missing []string
		for _, s := range requiredSections {
			if !strings.Contains(content, s.keyword) {
				missing = append(missing, s.role)
			}
		}
		if len(missing) > 0 {
			return HookResult{
				Passed:  false,
				Message: fmt.Sprintf("manual-test-plan-gate: missing role sections in manual-test-plan.md: %s", strings.Join(missing, ", ")),
			}
		}
		return HookResult{Passed: true}
	},
}

// defaultHooks returns the set of hooks active when no project-local override
// is present. The order determines execution sequence within each phase.
func defaultHooks() []Hook {
	return []Hook{
		// Pre-checkpoint (all): self-review
		selfReviewGate,
		// Post-checkpoint per stage (ordered to match pipeline flow)
		specCompletenessGate,      // spec
		adrQualityGate,            // arch
		archFileLint,              // arch
		tddGate,                   // test
		breakdownCompletenessGate, // breakdown
		taskCompletionGate,        // code
		securityHygieneGate,       // code
		specCodeAlignmentGateCode, // code  — spec-vs-code blocking gap check
		specCodeAlignmentGateQA,   // qa-verify — same check, runs first (zero tokens)
		manualTestPlanGate,        // qa-verify — 6-role manual test plan completeness
		qaCoverageGate,            // qa-verify
	}
}

// runHooks executes all applicable hooks for the given phase and checkpoint,
// returning a slice of failed results (empty = all passed).
//
// Pre-checkpoint hooks: gate="" hooks apply to all checkpoints.
// Post-checkpoint hooks: gate-specific hooks apply only to matching checkpoints.
func runHooks(phase HookPhase, ctx HookContext, hooks []Hook, cfg HookConfig) []HookResult {
	var failed []HookResult
	for _, h := range hooks {
		if h.Phase != phase {
			continue
		}
		if h.Gate != "" && h.Gate != ctx.CheckpointName {
			continue
		}
		if isHookDisabled(cfg, h.Name) {
			continue
		}
		res := h.Handler(ctx)
		if !res.Passed {
			failed = append(failed, res)
		}
	}
	return failed
}
