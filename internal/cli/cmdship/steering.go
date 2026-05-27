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

// steering.go — context steering injection for forge ship checkpoints.
//
// Steerings are short, targeted contextual instructions (< 300 tokens each)
// prepended to LLM system prompts at specific pipeline stages. They implement
// the "steering-injection" pattern from the forge knowledge base.
//
// Design rules:
//   - Each steering is ≤ 300 tokens.
//   - At most 3 steerings are injected per checkpoint (token budget).
//   - Steerings are deduplicated by Name before injection.
//   - Project-local overrides live in .forge/steerings/<name>.md.
//   - The "prompt-guide" steering is always-on (cannot be disabled via HookConfig).
//
// Default steerings (mapped to all 7 forge ship checkpoints):
//
//	prompt-guide              — ALL: behavioral standards (no placeholders, no hedging)
//	requirements-quality-scan — spec: Given/When/Then AC, measurable NFRs, impact analysis
//	tdd-standards             — test: TDD gate, no always-passing assertions, coverage targets
//	task-decomposition        — breakdown: atomic tasks, done criteria, effort sizing
//	implementation-standards  — code: OWASP hygiene, no secrets, sandbox paths
//	review-dab                — arch (full/light DAB), ship (significant change)
//	review-dab-light          — arch (warning/low-risk), ship (warning status)
//	review-tech-change        — ship (technical-change-only, single service)
//	qa-completeness           — qa-verify: AC coverage, all test types, metrics
//	handoff-protocol          — spec/arch/breakdown/code/test: cross-reference artefacts
package cmdship

import (
	"os"
	"path/filepath"
	"strings"
)

// Steering is a contextual prompt injection applied when Applies returns true.
type Steering struct {
	// Name uniquely identifies this steering (used for deduplication and overrides).
	Name string
	// Applies returns true when this steering should be injected for the given
	// checkpoint and current result state.
	Applies func(checkpoint string, result *Checkpoint) bool
	// Prompt is the text prepended to the LLM system prompt (≤ 300 tokens).
	Prompt string
}

// ── Built-in steering prompts ─────────────────────────────────────────────────

const promptGuideSteering = `Behavioral standards (always enforced):
1. No placeholder text (TODO, TBD, <fill-in>, "...", "example text").
2. No hedging language ("might", "could perhaps", "possibly", "maybe").
3. No scope creep — address only what was asked; flag extras as open questions.
4. All file references use relative paths from the project root.
5. When uncertain, flag as a gap with a hint; do not invent content.`

// ── spec ─────────────────────────────────────────────────────────────────────

const requirementsQualitySteering = `Requirements quality scan (spec checkpoint):
1. Every acceptance criterion must use Given/When/Then format.
2. NFRs must include measurable thresholds (e.g. "p95 latency < 200 ms", "error rate < 0.1%").
3. Perform impact analysis: list upstream/downstream services affected by this change.
4. Flag ambiguous, contradictory, or low-testability requirements as open questions.
5. Identify implicit dependencies on systems not mentioned in the description.`

// ── test ─────────────────────────────────────────────────────────────────────

const tddStandardsSteering = `TDD standards (test checkpoint):
1. Tests must be written BEFORE implementation code (red-green-refactor cycle).
2. No always-passing assertions: every test must be able to fail on a regression.
3. Mock only external I/O (DB, HTTP, filesystem); never mock the unit under test.
4. Cover all AC items: happy path + at least one boundary case + one negative case per criterion.
5. Coverage targets: ≥80% line coverage for new code; 100% for security-critical paths.
6. Test names must describe the scenario: "TestXxx_WhenY_ShouldZ".`

// ── breakdown ────────────────────────────────────────────────────────────────

const taskDecompositionSteering = `Task decomposition standards (breakdown checkpoint):
1. Each task must be atomic: completable by one engineer in ≤ 2 days.
2. Every task must have explicit done criteria (not just a description).
3. Tasks must be ordered by dependency — no task should block another unless listed as a dependency.
4. Effort sizing required: S (< 2h) / M (2–8h) / L (1–2d) per task.
5. Cross-reference the spec file for each task that implements an AC item.`

// ── code ─────────────────────────────────────────────────────────────────────

const implementationStandardsSteering = `Implementation standards (code checkpoint):
1. No secrets, tokens, or credentials in source files — use environment variables.
2. All filesystem paths validated via the project sandbox (no path traversal).
3. All subprocess execution goes through the allow-listed spawn utility; no shell=true.
4. OWASP Top 10: verify injection (A03), broken auth (A07), and insecure design (A04) are addressed.
5. New public APIs must have input validation at the boundary; internal callers may trust.`

// ── arch ─────────────────────────────────────────────────────────────────────

const reviewDABSteering = `Design Approval Board — Full DAB checklist (arch checkpoint):
1. At least 2 architectural alternatives evaluated with pros/cons (not just the chosen option).
2. Service decomposition: C4 context + container diagrams required for new services.
3. Security threat model: identify top-3 OWASP risks; document mitigations.
4. Data residency and privacy impact assessed; PII handling and retention documented.
5. Rollback / undo procedure documented and reversible within one deploy cycle.
6. Integration contracts (API, event schemas) versioned and backward-compatible.`

const reviewDABLightSteering = `Design Approval Board Light checklist (lower-risk / single-service change):
1. Change is bounded to a single service or module; no cross-service contract changes.
2. No new external dependencies introduced without an ADR.
3. Rollback is possible by reverting a single deployment unit.
4. Existing tests cover the change path; no coverage regression.
5. Sequence diagram required only if a new inter-service call is introduced.`

// ── ship ─────────────────────────────────────────────────────────────────────

const reviewTechChangeSteering = `Technical Change review (ship checkpoint — single-service, no arch change):
1. Confirm change is isolated: no schema migrations, no new service dependencies.
2. CAB Shift-Left checklist: rollback tested, feature-flagged if risky, changelog updated.
3. Observability: new code paths emit logs / metrics / traces at appropriate levels.
4. No breaking changes to public interfaces without a deprecation notice.`

// ── arch: scope-scan phase (runs before DAB type decision) ──────────────────

const archReadinessSteering = `Architecture readiness scan (arch checkpoint — Phase 1, scope assessment):
1. Identify all services, APIs, databases, and external systems touched by this change.
2. Classify change scope:
   • Cross-service / new service → Full DAB review required
   • Single-service bounded change → DAB Light
   • Config / ops / documentation only → Technical Change
3. Flag missing architecture context (no existing C4, no prior ADR) as blocking gaps.
4. List the top-3 OWASP risk categories that apply to this feature's domain.
5. Identify integration contracts that must remain backward-compatible.`

// ── qa-verify: manual test plan generation (Phase 3) ─────────────────────────

const manualTestPlanSteering = `Manual test plan standards (qa-verify checkpoint — Phase 3):
1. Each role section in manual-test-plan.md must be independently executable by a human tester.
2. Preconditions: list exact test data, environment setup, and required user roles (use placeholders for secrets).
3. Steps must be numbered with exact UI actions or API call payloads and expected results.
4. Edge cases: minimum 3 boundary/negative scenarios per AC item.
5. Security probes: attempt each OWASP Top-10 risk relevant to this feature; expected outcome is "BLOCKED" or "LOGGED".
6. Performance sanity: include at least one response-time assertion referencing NFR thresholds from spec.md.
7. Mark each step with a PASS / FAIL / SKIP verdict column and an acceptance threshold.`

// ── qa-verify ────────────────────────────────────────────────────────────────

const qaCompletenessSteering = `QA completeness gate (qa-verify checkpoint):
1. Every AC item from spec.md must be covered by at least one test case.
2. All test types present: unit + integration + at least one E2E or contract test.
3. No open defects in "blocker" or "critical" severity; all P1/P2 test failures resolved.
4. Performance baseline captured: p50/p95/p99 latency and error rate for critical paths.
5. Security scan passed: zero high-severity findings from automated scanner.`

// ── cross-checkpoint ─────────────────────────────────────────────────────────

const handoffProtocolSteering = `Handoff protocol for artefact references:
- Every output document must reference prior artefacts by file path (not inline copy).
- Example: "See .forge/specs/{feature}/spec.md §Acceptance Criteria"
- Cross-reference: spec→arch, arch→test, test→breakdown, breakdown→code.
- Do not duplicate content across documents; cross-reference instead.`

// ── Default steering registry ─────────────────────────────────────────────────

// DefaultSteerings returns the built-in steering set balanced across all 7
// forge ship checkpoints.
//
// Selection budget per checkpoint (max 3 per call):
//
//	spec:      prompt-guide + requirements-quality-scan + handoff-protocol
//	arch:      prompt-guide + arch-readiness (Phase 1) / review-dab (Phase 2+) + handoff-protocol
//	test:      prompt-guide + tdd-standards + handoff-protocol
//	breakdown: prompt-guide + task-decomposition + handoff-protocol
//	code:      prompt-guide + implementation-standards + handoff-protocol
//	ship:      prompt-guide + review-dab / review-dab-light / review-tech-change
//	qa-verify: prompt-guide + qa-completeness + manual-test-plan (Phase 3)
func DefaultSteerings() []Steering {
	return []Steering{
		// ── always-on ────────────────────────────────────────────────────────
		{
			Name:    "prompt-guide",
			Applies: func(_ string, _ *Checkpoint) bool { return true },
			Prompt:  promptGuideSteering,
		},
		// ── spec ─────────────────────────────────────────────────────────────
		{
			Name: "requirements-quality-scan",
			Applies: func(checkpoint string, _ *Checkpoint) bool {
				return checkpoint == "spec"
			},
			Prompt: requirementsQualitySteering,
		},
		// ── test ─────────────────────────────────────────────────────────────
		{
			Name: "tdd-standards",
			Applies: func(checkpoint string, _ *Checkpoint) bool {
				return checkpoint == "test"
			},
			Prompt: tddStandardsSteering,
		},
		// ── breakdown ────────────────────────────────────────────────────────
		{
			Name: "task-decomposition",
			Applies: func(checkpoint string, _ *Checkpoint) bool {
				return checkpoint == "breakdown"
			},
			Prompt: taskDecompositionSteering,
		},
		// ── code ─────────────────────────────────────────────────────────────
		{
			Name: "implementation-standards",
			Applies: func(checkpoint string, _ *Checkpoint) bool {
				return checkpoint == "code"
			},
			Prompt: implementationStandardsSteering,
		},
		// ── arch Phase 1: scope-scan before DAB type decision ─────────────────
		{
			Name: "arch-readiness",
			Applies: func(checkpoint string, result *Checkpoint) bool {
				// Inject on arch when no result yet (Phase 1 call, no prior state).
				return checkpoint == "arch" && result == nil
			},
			Prompt: archReadinessSteering,
		},
		// ── arch: full DAB (new services / cross-service changes) ─────────────
		{
			Name: "review-dab",
			Applies: func(checkpoint string, result *Checkpoint) bool {
				switch checkpoint {
				case "arch":
					// Full DAB unless result already flagged as low-risk warning.
					return result == nil || result.Status != "warning"
				case "ship":
					// Full DAB on ship only for significant (non-warning, non-ok-already) changes.
					return result == nil
				}
				return false
			},
			Prompt: reviewDABSteering,
		},
		// ── arch/ship: DAB Light (single-service, bounded change) ─────────────
		{
			Name: "review-dab-light",
			Applies: func(checkpoint string, result *Checkpoint) bool {
				switch checkpoint {
				case "arch":
					return result != nil && result.Status == "warning"
				case "ship":
					return result != nil && result.Status == "warning"
				}
				return false
			},
			Prompt: reviewDABLightSteering,
		},
		// ── ship: Technical Change (isolated, no arch impact) ─────────────────
		{
			Name: "review-tech-change",
			Applies: func(checkpoint string, result *Checkpoint) bool {
				// Applied to ship when result is already ok (low-friction path).
				return checkpoint == "ship" && result != nil && result.Status == "ok"
			},
			Prompt: reviewTechChangeSteering,
		},
		// ── qa-verify ────────────────────────────────────────────────────────
		{
			Name: "qa-completeness",
			Applies: func(checkpoint string, _ *Checkpoint) bool {
				return checkpoint == "qa-verify"
			},
			Prompt: qaCompletenessSteering,
		},
		// ── qa-verify Phase 3: manual test plan generation ────────────────────
		{
			Name: "manual-test-plan",
			Applies: func(checkpoint string, _ *Checkpoint) bool {
				return checkpoint == "qa-verify"
			},
			Prompt: manualTestPlanSteering,
		},
		// ── handoff: all document-producing checkpoints ───────────────────────
		{
			Name: "handoff-protocol",
			Applies: func(checkpoint string, _ *Checkpoint) bool {
				switch checkpoint {
				case "spec", "arch", "test", "breakdown", "code":
					return true
				}
				return false
			},
			Prompt: handoffProtocolSteering,
		},
	}
}

// ── Steering application ───────────────────────────────────────────────────────

// maxSteeringsPerCheckpoint caps how many steerings are injected per LLM call.
const maxSteeringsPerCheckpoint = 3

// selectSteerings returns up to maxSteeringsPerCheckpoint applicable steerings
// for the given checkpoint and result state, deduplicated by Name.
func selectSteerings(checkpoint string, result *Checkpoint, steerings []Steering) []Steering {
	seen := make(map[string]bool)
	var selected []Steering
	for _, s := range steerings {
		if seen[s.Name] {
			continue
		}
		if s.Applies(strings.ToLower(checkpoint), result) {
			selected = append(selected, s)
			seen[s.Name] = true
			if len(selected) >= maxSteeringsPerCheckpoint {
				break
			}
		}
	}
	return selected
}

// applySteerings prepends the selected steerings to a system prompt.
// The combined result never exceeds maxSystemTokens (approximate char budget).
// Each steering is separated by a blank line.
func applySteerings(system string, steerings []Steering, maxSystemChars int) string {
	if len(steerings) == 0 {
		return system
	}
	var parts []string
	for _, s := range steerings {
		// Check project-local override first.
		parts = append(parts, s.Prompt)
	}
	prefix := strings.Join(parts, "\n\n") + "\n\n"
	combined := prefix + system
	if maxSystemChars > 0 && len(combined) > maxSystemChars {
		// Trim the system portion (not the steerings) to stay within budget.
		headroom := maxSystemChars - len(prefix)
		if headroom > 0 && headroom < len(system) {
			combined = prefix + system[:headroom]
		} else if headroom <= 0 {
			combined = prefix // extreme budget pressure: steerings only
		}
	}
	return combined
}

// loadProjectSteering returns the content of .forge/steerings/<name>.md if it
// exists, otherwise returns "". This enables per-project overrides.
func loadProjectSteering(root, name string) string {
	p := filepath.Join(root, ".forge", "steerings", name+".md")
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// resolveSteeringPrompt returns the effective prompt for a steering: project-local
// override when present, built-in text otherwise.
func resolveSteeringPrompt(root string, s Steering) string {
	if override := loadProjectSteering(root, s.Name); override != "" {
		return override
	}
	return s.Prompt
}

// BuildSystemWithSteerings constructs an LLM system prompt by applying the
// applicable steerings from defaultSteerings() to the base system string.
//
// This is the single call-site helper used by checkpoint functions that need
// steering-enriched prompts without managing the steering lifecycle themselves.
func BuildSystemWithSteerings(root, checkpoint, baseSystem string, result *Checkpoint) string {
	steerings := selectSteerings(checkpoint, result, DefaultSteerings())
	// Resolve project-local overrides.
	resolved := make([]Steering, len(steerings))
	for i, s := range steerings {
		resolved[i] = Steering{
			Name:    s.Name,
			Applies: s.Applies,
			Prompt:  resolveSteeringPrompt(root, s),
		}
	}
	// ~16000 chars ≈ ~4000 tokens; generous budget for system prompts.
	return applySteerings(baseSystem, resolved, 16000)
}
