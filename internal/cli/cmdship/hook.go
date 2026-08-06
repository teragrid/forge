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
//	PhasePreCheckpoint  — declared, but NOT CURRENTLY INVOKED. runWithOptions
//	                      only calls runHooks for the two phases below, so
//	                      self-review-gate — the sole pre-checkpoint hook — has
//	                      never executed in the pipeline. See
//	                      TestPreCheckpointHooks_AreRegisteredButNeverRun.
//	PhasePostCheckpoint — runs after the checkpoint completes; a VerdictFail
//	                      result flags the checkpoint as "warning" (non-blocking
//	                      by default; set HookConfig.Strict to elevate to "fail").
//	                      A VerdictUnknown result annotates the checkpoint
//	                      UNVERIFIED and never escalates it.
//	PhasePostPipeline   — runs once after all checkpoints pass; used for
//	                      learning extraction, review routing, and KB updates.
//	                      Failures here are always advisory-only (see the
//	                      "post-pipeline failures are advisory only" comment
//	                      at the call site in ship.go) — there is no
//	                      checkpoint left to fail.
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
//	four-stage-testing-gate   — post-checkpoint (qa-verify): testing-pipeline.md evidence
//	                            present for all 4 stages. Blocking by default since
//	                            1.8.2; waive with `forge ship --no-strict-testing` or
//	                            "strict-testing: false" in .forge/hooks.yaml.
//	                            See testing_pipeline.go.
//	four-stage-testing-reminder — post-pipeline: always prints the 4-stage testing
//	                            pipeline checklist to stderr after a successful run,
//	                            regardless of StrictTesting. Pure reminder, never blocks.
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
	// SpecName is the --name/-n override, when the caller resolved one. Takes
	// priority over the slug derived from Description — see auditSpecVsCode.
	SpecName string
	// Pipe is the active LLM pipe (may be nil in dry-run / no-LLM mode).
	Pipe *LLMPipe
	// Result is the checkpoint result; nil during PhasePreCheckpoint.
	Result *Checkpoint
	// StrictTesting mirrors HookConfig.StrictTesting for this run — resolved
	// once in runWithOptions (file OR --strict-testing flag) and copied into
	// every HookContext so a handler doesn't need its own config lookup.
	StrictTesting bool
}

// Verdict is the outcome of a quality gate. It has three states, not two, and
// that is the whole point of the type.
//
// A bool forces every gate that *could not check* — artefact missing, tool not
// installed, config it cannot parse — to answer either "pass" or "fail". Gate
// authors almost always pick pass, because failing a build over something that
// is not the user's fault is obviously wrong. The result is that "I did not
// verify this" and "I verified this and it is fine" become the same value, and
// the caller cannot tell them apart. Every instance of that in forge has been
// the same bug: a green checkpoint standing on a check that never ran.
//
// VerdictUnknown is deliberately the zero value. A handler that forgets to set
// a verdict yields "unverified", which is honest, rather than falling into
// either a false pass or a spurious build failure.
type Verdict int

const (
	// VerdictUnknown means the gate could not determine an answer. It is never
	// treated as passing.
	VerdictUnknown Verdict = iota
	// VerdictPass means the gate checked and found no issue.
	VerdictPass
	// VerdictFail means the gate checked and found an issue.
	VerdictFail
)

// String renders the verdict for logs and checkpoint details.
func (v Verdict) String() string {
	switch v {
	case VerdictPass:
		return "pass"
	case VerdictFail:
		return "fail"
	default:
		return "unverified"
	}
}

// HookResult is what a hook handler returns.
type HookResult struct {
	// Verdict is the gate's outcome. The zero value is VerdictUnknown.
	Verdict Verdict
	// Message is the human-readable explanation. Required for VerdictFail and
	// VerdictUnknown — a result the user cannot act on is barely better than
	// no gate at all. Empty for VerdictPass.
	Message string
	// HookName is set by runHooks (not the handler) to the originating
	// Hook.Name — lets a caller distinguish which specific hook failed
	// without every Handler needing to know its own registered name. Used
	// by ship.go to escalate a four-stage-testing-gate failure via
	// HookConfig.StrictTesting without also escalating unrelated
	// same-checkpoint hook failures that only HookConfig.Strict governs.
	HookName string
}

// gatePass reports that the gate checked and found no issue.
func gatePass() HookResult { return HookResult{Verdict: VerdictPass} }

// gateFail reports that the gate checked and found an issue.
func gateFail(format string, args ...any) HookResult {
	return HookResult{Verdict: VerdictFail, Message: fmt.Sprintf(format, args...)}
}

// gateUnknown reports that the gate could not check.
//
// Use this — never gatePass — when the artefact is missing, a tool is absent,
// or input cannot be parsed. Saying "pass" there is the bug this whole type
// exists to prevent: it launders an unexamined state into a verified one, and
// the checkpoint goes green on a check that never ran.
func gateUnknown(format string, args ...any) HookResult {
	return HookResult{Verdict: VerdictUnknown, Message: fmt.Sprintf(format, args...)}
}

// gateNotApplicable is the one case where "we did not check" is uninteresting:
// the checkpoint has already failed, so its gates are moot. It is Unknown like
// any other unchecked state, but the caller suppresses it on a failed
// checkpoint rather than piling "unverified" notes onto a run that already has
// a real error to report.
func gateNotApplicable() HookResult {
	return HookResult{Verdict: VerdictUnknown, Message: "checkpoint already failed; gate not evaluated"}
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
	// StrictTesting escalates four-stage-testing-gate specifically (missing
	// .forge/specs/<slug>/testing-pipeline.md evidence) from advisory-only to
	// a blocking checkpoint failure. Deliberately independent of Strict —
	// enabling it must not also make every other hook in defaultHooks()
	// blocking.
	//
	// Default: TRUE as of 1.8.2. Testing evidence is not a premium feature of
	// forge, it is the product: a pipeline that ships a change while quietly
	// noting that nobody verified it is doing the one thing forge exists to
	// prevent. Advisory-by-default meant the gate's own finding — "there is no
	// evidence this was tested" — was itself reported as an acceptable outcome.
	//
	// Turn it off per-project with "strict-testing: false" in
	// .forge/hooks.yaml, or per-run with `forge ship --no-strict-testing`.
	StrictTesting bool
}

// defaultHookConfig is the configuration used when .forge/hooks.yaml is absent.
// Every field here is a policy decision, so they are stated in one place rather
// than relying on Go's zero values — a zero value silently becomes policy, and
// "strict testing is off" is not something that should be expressible by
// accident.
func defaultHookConfig() HookConfig {
	return HookConfig{
		Strict:        false, // unrelated hooks stay advisory; see Strict
		StrictTesting: true,  // 1.8.2: evidence required unless explicitly waived
	}
}

// loadHookConfig reads .forge/hooks.yaml if it exists.
// Missing file → defaultHookConfig().
func loadHookConfig(root string) HookConfig {
	cfg := defaultHookConfig()
	path := filepath.Join(root, ".forge", "hooks.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	// Minimal YAML parsing: only "disabled:", "strict:" and "strict-testing:"
	// are supported. "strict-testing" is matched before "strict" so a
	// hyphenated key is never mistaken for the plain one.
	//
	// Both true and false are now accepted for strict-testing. Before 1.8.2
	// only "true" was read, because the default was false and the file could
	// therefore only ever turn the gate ON. With the default inverted, an
	// unreadable "false" would mean a project had no way to opt out at all —
	// which would make the gate a mandate rather than a default.
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "strict-testing: true"):
			cfg.StrictTesting = true
		case strings.HasPrefix(line, "strict-testing: false"):
			cfg.StrictTesting = false
		case strings.HasPrefix(line, "strict: true"):
			cfg.Strict = true
		case strings.HasPrefix(line, "strict: false"):
			cfg.Strict = false
		case strings.HasPrefix(line, "- "):
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
			return gateUnknown("self-review-gate: no known artefact for checkpoint %q — nothing scanned",
				ctx.CheckpointName)
		}

		badPatterns := []string{"TODO", "TBD", "<fill", "...", "might consider", "could perhaps"}
		scanned := 0
		for _, fp := range filesToScan {
			data, err := os.ReadFile(fp)
			if err != nil {
				continue // file not yet written — nothing to scan
			}
			scanned++
			content := string(data)
			for _, pat := range badPatterns {
				if strings.Contains(content, pat) {
					return gateFail("self-review-gate: placeholder/hedging in %s: %q", filepath.Base(fp), pat)
				}
			}
		}
		// Scanning zero files is not a clean bill of health. On a first run the
		// artefact does not exist yet — the checkpoint is about to write it —
		// so this is the gate's normal state rather than an anomaly, which is
		// exactly why reporting it as "pass" was so easy to leave in place.
		if scanned == 0 {
			return gateUnknown("self-review-gate: no artefact present for %q — nothing scanned",
				ctx.CheckpointName)
		}
		return gatePass()
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
			return gateNotApplicable()
		}
		slug := slugify(ctx.Description)
		specPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "spec.md")
		data, err := os.ReadFile(specPath)
		if err != nil {
			return gateUnknown("spec-completeness-gate: spec.md not found — acceptance criteria unverified")
		}
		content := string(data)
		// Must have an Acceptance Criteria / AC section.
		hasACSection := strings.Contains(strings.ToLower(content), "acceptance criteria") ||
			strings.Contains(strings.ToLower(content), "## ac")
		if !hasACSection {
			return gateFail("spec-completeness-gate: spec.md missing Acceptance Criteria section")
		}
		// At least one Given/When/Then criterion.
		hasGWT := strings.Contains(content, "Given ") &&
			strings.Contains(content, "When ") &&
			strings.Contains(content, "Then ")
		if !hasGWT {
			return gateFail("spec-completeness-gate: spec.md AC missing Given/When/Then format")
		}
		return gatePass()
	},
}

// taskCompletionGate checks that every targeted task in tasks.md is marked done.
var taskCompletionGate = Hook{
	Name:  "task-completion-gate",
	Phase: PhasePostCheckpoint,
	Gate:  "code",
	Handler: func(ctx HookContext) HookResult {
		if ctx.Result == nil || ctx.Result.Status == "fail" {
			return gateNotApplicable()
		}
		slug := slugify(ctx.Description)
		tasksPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "tasks.md")
		data, err := os.ReadFile(tasksPath)
		if err != nil {
			return gateUnknown("task-completion-gate: tasks.md not found — task completion unverified")
		}
		lines := strings.Split(string(data), "\n")
		incomplete := 0
		for _, l := range lines {
			if strings.Contains(l, "- [ ]") {
				incomplete++
			}
		}
		if incomplete > 0 {
			return gateFail("task-completion-gate: %d incomplete task(s) remain in tasks.md after code checkpoint", incomplete)
		}
		return gatePass()
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
			return gateNotApplicable()
		}
		slug := slugify(ctx.Description)
		adrPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "adr.md")
		data, err := os.ReadFile(adrPath)
		if err != nil {
			return gateUnknown("adr-quality-gate: adr.md not found — architecture decision unverified")
		}
		content := strings.ToLower(string(data))
		// Look for at least 2 alternative headings or list items.
		altCount := strings.Count(content, "alternative") + strings.Count(content, "option ")
		if altCount < 2 {
			return gateFail("adr-quality-gate: ADR must evaluate ≥2 alternatives; found fewer markers")
		}
		if !strings.Contains(content, "consequence") && !strings.Contains(content, "trade-off") {
			return gateFail("adr-quality-gate: ADR missing consequences/trade-offs section")
		}
		return gatePass()
	},
}

// archFileLint scans architecture artefacts for empty sections and TODO markers.
var archFileLint = Hook{
	Name:  "arch-file-lint",
	Phase: PhasePostCheckpoint,
	Gate:  "arch",
	Handler: func(ctx HookContext) HookResult {
		if ctx.Result == nil || ctx.Result.Status == "fail" {
			return gateNotApplicable()
		}
		slug := slugify(ctx.Description)
		archPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "arch.md")
		data, err := os.ReadFile(archPath)
		if err != nil {
			return gateUnknown("arch-file-lint: arch.md not found — architecture document unverified")
		}
		// Detect consecutive heading followed immediately by another heading
		// (empty section) or a TODO marker.
		lines := strings.Split(string(data), "\n")
		for i, l := range lines {
			trimmed := strings.TrimSpace(l)
			if strings.HasPrefix(trimmed, "#") && i+1 < len(lines) {
				next := strings.TrimSpace(lines[i+1])
				if strings.HasPrefix(next, "#") {
					return gateFail("arch-file-lint: empty section detected after %q", trimmed)
				}
			}
			if strings.Contains(trimmed, "TODO") || strings.Contains(trimmed, "TBD") {
				return gateFail("arch-file-lint: placeholder detected in arch.md: %q", trimmed)
			}
		}
		return gatePass()
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
			return gateNotApplicable()
		}
		slug := slugify(ctx.Description)
		testsPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "tests.md")
		data, err := os.ReadFile(testsPath)
		if err != nil {
			return gateUnknown("tdd-gate: tests.md not found — test quality unverified")
		}
		content := string(data)
		// Detect always-passing anti-patterns.
		alwaysPass := []string{
			"assert(true)", "assertTrue(true)", "expect(true).toBe(true)",
			"assert.True(t, true)", "t.Skip(", "// TODO: test",
		}
		for _, pat := range alwaysPass {
			if strings.Contains(content, pat) {
				return gateFail("tdd-gate: always-passing or skipped test pattern detected: %q", pat)
			}
		}
		// Must reference at least one Given/When/Then or test scenario.
		if !strings.Contains(content, "Given ") && !strings.Contains(content, "Scenario:") &&
			!strings.Contains(content, "func Test") {
			return gateFail("tdd-gate: tests.md must contain at least one test scenario (Given/When/Then or func Test*)")
		}
		return gatePass()
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
			return gateNotApplicable()
		}
		slug := slugify(ctx.Description)
		tasksPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "tasks.md")
		data, err := os.ReadFile(tasksPath)
		if err != nil {
			return gateUnknown("breakdown-completeness: tasks.md not found — breakdown unverified")
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
			return gateFail("breakdown-completeness: %d task(s) have empty descriptions in tasks.md", empty)
		}
		return gatePass()
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
			return gateNotApplicable()
		}
		slug := slugify(ctx.Description)
		implPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "impl-notes.md")
		data, err := os.ReadFile(implPath)
		if err != nil {
			return gateUnknown("security-hygiene-gate: impl-notes.md not found — implementation notes unscanned")
		}
		content := string(data)
		// Secret-like patterns.
		secretPatterns := []string{
			"password =", "api_key =", "apikey =", "secret =",
			"token =", "private_key", "AWS_SECRET", "-----BEGIN",
		}
		for _, pat := range secretPatterns {
			if strings.Contains(strings.ToLower(content), strings.ToLower(pat)) {
				return gateFail("security-hygiene-gate: potential secret pattern %q in impl-notes.md", pat)
			}
		}
		// Shell injection indicators.
		shellPatterns := []string{"shell=true", "os.system(", "exec.Command(\"sh\",", "exec.Command(\"bash\","}
		for _, pat := range shellPatterns {
			if strings.Contains(content, pat) {
				return gateFail("security-hygiene-gate: shell-injection risk: %q found in impl-notes.md", pat)
			}
		}
		return gatePass()
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
			return gateNotApplicable()
		}
		slug := slugify(ctx.Description)
		specPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "spec.md")
		qaPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "qa-report.md")

		specData, specErr := os.ReadFile(specPath)
		qaData, qaErr := os.ReadFile(qaPath)
		if specErr != nil || qaErr != nil {
			return gateUnknown("qa-coverage-gate: spec.md or qa-report.md not found — AC coverage unverified")
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
			return gateFail("qa-coverage-gate: %d/%d AC items referenced in qa-report.md", covered, acCount)
		}
		return gatePass()
	},
}

// specCodeAlignmentHandler is the shared logic for specCodeAlignmentGateCode
// and specCodeAlignmentGateQA — wraps auditSpecVsCode() and fails the gate when
// blocking gaps are found (incomplete tasks, untested authz roles, missing event tests).
var specCodeAlignmentHandler = func(ctx HookContext) HookResult {
	if ctx.Result == nil || ctx.Result.Status == "fail" {
		return gateNotApplicable()
	}
	result := auditSpecVsCode(ctx.Root, ctx.Description, ctx.SpecName)
	// Without spec.md there is nothing to align code against, and auditSlug()
	// returns early — so every gap check below is skipped. This used to fall
	// through to "pass", which meant `forge ship --from=code` on a project
	// whose spec was never written got a green alignment gate that had
	// verified nothing. Found by the gate mutation table (M2); this is what
	// having a third verdict is for.
	if !result.SpecFound {
		return gateUnknown("spec-code-alignment-gate: spec.md not found — spec-vs-code alignment unverified")
	}
	if !result.HasBlockingGaps() {
		return gatePass()
	}
	var msgs []string
	for _, g := range result.Gaps {
		if g.Severity == "blocking" {
			msgs = append(msgs, fmt.Sprintf("[%s] %s (hint: %s)", g.Type, g.Description, g.Hint))
		}
	}
	return gateFail("spec-code-alignment-gate: %d blocking gap(s): %s", len(msgs), strings.Join(msgs, "; "))
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
			return gateNotApplicable()
		}
		slug := slugify(ctx.Description)
		planPath := filepath.Join(ctx.Root, ".forge", "specs", slug, "manual-test-plan.md")
		data, err := os.ReadFile(planPath)
		if err != nil {
			return gateFail("manual-test-plan-gate: manual-test-plan.md not found — run qa-verify with an LLM configured")
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
			return gateFail("manual-test-plan-gate: missing role sections in manual-test-plan.md: %s", strings.Join(missing, ", "))
		}
		return gatePass()
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
		fourStageTestingGate,      // qa-verify — local/pre-push/staging/production evidence
		// Post-pipeline (once, after all checkpoints pass)
		fourStageTestingReminder,
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
		res.HookName = h.Name
		if res.Verdict != VerdictPass {
			failed = append(failed, res)
			continue
		}
		// A gate that returns Pass has read an artefact off disk and
		// re-validated it — the definition of read-back evidence (M1). This is
		// where most checkpoints earn their green: the gates were already
		// doing the verifying, it was simply never recorded as the basis for
		// the status.
		//
		// Only VerdictPass qualifies. Unknown means the gate could not check,
		// and M3 exists precisely so that no longer counts for anything.
		//
		// ctx.Result is nil in the pre-checkpoint phase and AddEvidence is a
		// no-op on nil, which is the behaviour we want: a pre-checkpoint scan
		// examines the *previous* run's artefact, so it is not evidence about
		// what this run is about to produce.
		ctx.Result.AddEvidence(SourceReadBack, h.Name+" verified "+ctx.CheckpointName, "gate passed")
	}
	return failed
}

// partitionResults splits hook results into the two groups the caller treats
// differently: gates that found a real problem, and gates that could not check.
//
// Keeping them apart is the entire point of Verdict. Merging them would put
// "spec.md not found, nothing verified" and "spec.md is missing its acceptance
// criteria" into one bucket, which is how the two became indistinguishable in
// the first place.
func partitionResults(results []HookResult) (failures, unverified []HookResult) {
	for _, r := range results {
		switch r.Verdict {
		case VerdictFail:
			failures = append(failures, r)
		case VerdictUnknown:
			unverified = append(unverified, r)
		}
	}
	return failures, unverified
}
