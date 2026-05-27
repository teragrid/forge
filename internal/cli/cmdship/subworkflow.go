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

// subworkflow.go — declarative sub-workflow phase map for all 7 forge ship checkpoints.
//
// Design
// ──────
// Each checkpoint in the forge ship pipeline is composed of named Phases. A Phase
// declares:
//
//   - which KB family + tags to forward to InvokeWithKnowledge (enriches LLM context
//     from forge-knowledge/knowledge-base/ entries scored by checkpoint+family+tags)
//   - which role personas participate in debate for this phase
//   - how many debate rounds to run (0 = no debate, just single LLM call)
//   - which steering to inject into the system prompt
//   - which quality-gate hook guards the phase output
//   - whether the phase is fully deterministic (no LLM call needed)
//
// This file is intentionally declarative — no execution logic lives here.
// Steering selection, hook routing, and KB-enriched InvokeWithKnowledge calls
// all consume this map at their respective call sites.
//
// KB family taxonomy (matches forge-knowledge/knowledge-base/ scoring):
//
//	""           — score by tags only; pulls across all KB categories
//	"unit"       — entries with scan_families: [unit] (test design, quality gates)
//	"security"   — entries with scan_families: [security] (OWASP, authN/Z, secrets)
//	"reliability"— entries with scan_families: [reliability] (resilience patterns, SRE)
//	"compliance" — entries with scan_families: [compliance] (PCI-DSS, GDPR, Basel)
//
// Token budget per checkpoint (all phases combined):
//
//	spec:      ~1 500 tokens  (1 det + 2 KB-enriched LLM calls)
//	arch:      ~4 000 tokens  (1 det + 1 LLM×3 rounds (6 roles) + 1 focused sec scan)
//	test:      ~2 500 tokens  (2 KB-enriched LLM calls + 2-round debate)
//	breakdown: ~1 500 tokens  (1 KB-enriched + 1-round debate)
//	code:      ~2 000 tokens  (2 KB-enriched LLM calls + 2 det gates)
//	ship:      ~2 000 tokens  (1 det + 2 KB-enriched LLM calls + 1-round debate)
//	qa-verify: ~5 000 tokens  (1 det + 6 role LLM calls + 1 cross-challenge + gates)
package cmdship

// Phase describes one named phase within a checkpoint sub-workflow.
type Phase struct {
	// Name uniquely identifies this phase (format: "<checkpoint>/<phase>").
	Name string

	// ModelTier selects which model tier to use for this phase's LLM calls.
	// "tier-1" = heavy reasoning model (arch debate, security gates, qa-verify).
	// "tier-2" = fast structured-extraction model (spec, test, breakdown, synthesis).
	// "" = provider default.
	ModelTier string

	// KBCheckpoint is forwarded to InvokeWithKnowledge as the checkpoint param.
	// Used by knowledge.Score to weight ship_checkpoints matches.
	KBCheckpoint string

	// KBFamily is forwarded to InvokeWithKnowledge as the family param.
	// Drives scan_families scoring in knowledge.Select. Empty = score by tags only.
	KBFamily string

	// KBTags are forwarded to InvokeWithKnowledge as the tags slice.
	// Each matching tag in a KB entry adds +1 to its relevance score.
	KBTags []string

	// Roles lists the RoleID personas whose debate perspectives enrich this phase.
	// Empty = no multi-role debate for this phase.
	Roles []RoleID

	// DebateRounds is how many multi-role debate rounds to run (0 = none).
	// Each round: roles independently review, then cross-challenge each other.
	DebateRounds int

	// Steering is the name of the steering injected for this phase's LLM call.
	// Must match a Name in defaultSteerings(). Empty = no extra steering.
	Steering string

	// HookGate is the name of the hook that guards this phase's output.
	// Must match a Name in defaultHooks(). Empty = no gate.
	HookGate string

	// Deterministic marks a phase that performs only file/static analysis —
	// no LLM call is made. HookGate is still evaluated.
	Deterministic bool
}

// Model tier constants for Phase.ModelTier (L3: Model Tier Routing).
const (
	// ModelTierHeavy selects the most capable / reasoning-optimised model.
	// Use for: arch debate, code generation, security gates, qa-verify.
	ModelTierHeavy = "tier-1"

	// ModelTierFast selects the fastest / cheapest model.
	// Use for: spec intake, test design, breakdown, debate synthesis,
	// context digest generation.
	ModelTierFast = "tier-2"
)

// defaultSubWorkflows returns the sub-workflow phase map for all 7 checkpoints.
// Keys are lower-cased checkpoint names matching the pipeline index in ship.go.
//
// Alignment with forge-kb role model:
//
//	spec:      BA + PO + SA (concurrent requirements scan + impact analysis)
//	arch:      sa-orchestrator + 6 arch roles (scope scan → DAB design → sec scan)
//	test:      QE (full test spectrum) + BA (business rules) + Sec (security tests)
//	breakdown: DL (delivery) + SA (architecture) + QE (testability)
//	code:      DL + Ops (implementation) + Sec (security scan) + det gap check
//	ship:      Ops + CPO (compliance) + Sec (CAB Shift-Left)
//	qa-verify: det gap audit + 6-role manual plan (PO/BA/QE/Sec/Ops/CPO) + gates
func defaultSubWorkflows() map[string][]Phase {
	return map[string][]Phase{

		// ── spec ──────────────────────────────────────────────────────────────
		// Roles: BA (rules), PO (value), SA (architecture), CPO (compliance)
		// Goal: workspace context → requirements intake + impact analysis + completeness gate
		"spec": {
			{
				// Phase 0 (G9): deterministic workspace scan — zero LLM tokens.
				// Collects tech stack, top-level structure, recent git log, existing
				// specs, and project conventions into workspace-context.md.
				// Must run FIRST so that spec/intake and spec/impact-analysis have
				// concrete project context in their user prompts.
				Name:          "spec/workspace-context",
				Deterministic: true,
			},
			{
				Name:         "spec/intake",
				ModelTier:    ModelTierFast,
				KBCheckpoint: "spec",
				KBFamily:     "unit",
				KBTags:       []string{"requirements", "test-design", "quality-gate", "acceptance-criteria"},
				Roles:        []RoleID{RolePO, RoleBA},
				Steering:     "requirements-quality-scan",
			},
			{
				Name:         "spec/impact-analysis",
				ModelTier:    ModelTierFast,
				KBCheckpoint: "spec",
				KBFamily:     "compliance",
				KBTags:       []string{"gdpr", "pci-dss", "data-residency", "privacy", "pii"},
				Roles:        []RoleID{RoleSA, RoleCPO},
				DebateRounds: 1,
				Steering:     "handoff-protocol",
			},
			{
				Name:          "spec/completeness-gate",
				Deterministic: true,
				HookGate:      "spec-completeness-gate",
			},
		},

		// ── arch ──────────────────────────────────────────────────────────────
		// Roles: SysArch + SecArch + DatArch + APIDesign + PerfEng + PlatArch (3 rounds)
		// Goal: scope scan → DAB-type decision → 3-round design debate → security scan
		"arch": {
			{
				Name:         "arch/scope-scan",
				ModelTier:    ModelTierFast,
				KBCheckpoint: "arch",
				KBFamily:     "",
				KBTags:       []string{"architecture", "modularity", "cloud-native", "service-decomposition"},
				Roles:        []RoleID{RoleSysArch},
				Steering:     "arch-readiness",
			},
			{
				// Full design debate — 3 rounds covering C4, threat model, data model, API contracts.
				// Steering switches to review-dab-light automatically when checkArch detects low-risk.
				Name:         "arch/design",
				ModelTier:    ModelTierHeavy,
				KBCheckpoint: "arch",
				KBFamily:     "security",
				KBTags:       []string{"security", "resilience", "api-design", "data", "zero-trust", "threat-model"},
				Roles:        []RoleID{RoleSysArch, RoleSecArch, RoleDatArch, RoleAPIDesign, RolePerfEng, RolePlatArch},
				DebateRounds: 3,
				Steering:     "review-dab",
			},
			{
				// Targeted security architecture scan after design debate.
				Name:         "arch/security-scan",
				ModelTier:    ModelTierHeavy,
				KBCheckpoint: "arch",
				KBFamily:     "security",
				KBTags:       []string{"owasp", "mtls", "secrets", "zero-trust", "vault", "least-privilege"},
				Roles:        []RoleID{RoleSecArch},
				Steering:     "review-dab",
				HookGate:     "adr-quality-gate",
			},
			{
				Name:          "arch/lint-gate",
				Deterministic: true,
				HookGate:      "arch-file-lint",
			},
		},

		// ── test ──────────────────────────────────────────────────────────────
		// Roles: QE (full spectrum), BA (business rules), Sec (security tests)
		// Goal: full-spectrum test design + security test scenarios + TDD gate
		"test": {
			{
				// QE covers ALL test types — not just unit. KB tags reflect the full pyramid.
				Name:         "test/design",
				ModelTier:    ModelTierFast,
				KBCheckpoint: "test",
				KBFamily:     "",
				KBTags: []string{
					"test-design", "quality-gate", "tdd",
					"integration-testing", "e2e", "regression",
					"contract-testing", "mutation-testing", "smoke-testing",
					"acceptance-testing", "exploratory-testing", "performance-testing",
					"test-pyramid",
				},
				Roles:    []RoleID{RoleQE, RoleBA},
				Steering: "tdd-standards",
			},
			{
				// Security-specific test scenarios: injection, authz bypass, audit coverage.
				Name:         "test/security-test-design",
				ModelTier:    ModelTierFast,
				KBCheckpoint: "test",
				KBFamily:     "security",
				KBTags:       []string{"owasp", "authz", "audit-logging", "rls", "injection"},
				Roles:        []RoleID{RoleQE, RoleSec},
				DebateRounds: 2,
				Steering:     "handoff-protocol",
				HookGate:     "tdd-gate",
			},
		},

		// ── breakdown ─────────────────────────────────────────────────────────
		// Roles: DL (delivery lead), SA (architecture), QE (testability)
		// Goal: dependency-ordered atomic tasks + testability debate + completeness gate
		"breakdown": {
			{
				Name:         "breakdown/planning-scan",
				ModelTier:    ModelTierFast,
				KBCheckpoint: "breakdown",
				KBFamily:     "",
				KBTags:       []string{"architecture", "dependency", "delivery", "capacity-planning"},
				Roles:        []RoleID{RoleDL, RoleSA},
				Steering:     "task-decomposition",
			},
			{
				// QE challenges: "Is every task independently testable?"
				// SA challenges: "Does task order respect service contracts?"
				Name:         "breakdown/testability-debate",
				ModelTier:    ModelTierFast,
				KBCheckpoint: "breakdown",
				KBFamily:     "",
				KBTags:       []string{"estimation", "agile", "done-criteria", "test-pyramid"},
				Roles:        []RoleID{RoleDL, RoleSA, RoleQE},
				DebateRounds: 1,
				Steering:     "handoff-protocol",
				HookGate:     "breakdown-completeness",
			},
		},

		// ── code ──────────────────────────────────────────────────────────────
		// Roles: DL + Ops (implementation), Sec (security scan)
		// Goal: resilience-aware impl guidance → OWASP security scan → spec-code gap
		"code": {
			{
				// Pull resilience + observability patterns from KB for implementation guidance.
				Name:         "code/implementation",
				ModelTier:    ModelTierFast,
				KBCheckpoint: "code",
				KBFamily:     "reliability",
				KBTags:       []string{"resilience", "observability", "retry", "timeout", "circuit-breaker", "bulkhead"},
				Roles:        []RoleID{RoleDL, RoleOps},
				Steering:     "implementation-standards",
			},
			{
				// Dedicated security scan: OWASP + secrets + sandbox path validation.
				Name:         "code/security-scan",
				ModelTier:    ModelTierHeavy,
				KBCheckpoint: "code",
				KBFamily:     "security",
				KBTags:       []string{"owasp", "secrets", "vault", "injection", "sandbox", "least-privilege"},
				Roles:        []RoleID{RoleSec},
				Steering:     "implementation-standards",
				HookGate:     "security-hygiene-gate",
			},
			{
				// Deterministic: auditSpecVsCode() checks tasks + authz roles + event coverage.
				Name:          "code/spec-code-gap",
				Deterministic: true,
				HookGate:      "spec-code-alignment-gate",
			},
			{
				Name:          "code/task-completion-gate",
				Deterministic: true,
				HookGate:      "task-completion-gate",
			},
		},

		// ── ship ──────────────────────────────────────────────────────────────
		// Roles: CPO (compliance), Ops (deployment), Sec (CAB Shift-Left)
		// Goal: compliance scan → CAB review (DAB-type appropriate) → release gate
		"ship": {
			{
				Name:          "ship/lint-scan",
				Deterministic: true,
				HookGate:      "self-review-gate",
			},
			{
				// Pull PCI-DSS, GDPR, audit-logging KB entries for compliance attestation.
				Name:         "ship/compliance-scan",
				ModelTier:    ModelTierFast,
				KBCheckpoint: "ship",
				KBFamily:     "compliance",
				KBTags:       []string{"pci-dss", "gdpr", "sox", "audit-logging", "compliance"},
				Roles:        []RoleID{RoleCPO, RoleOps},
				Steering:     "review-tech-change",
			},
			{
				// CAB Shift-Left: Ops challenges deployment risk, Sec challenges attack surface,
				// CPO challenges data handling. Steering resolves to review-dab / review-dab-light
				// / review-tech-change at runtime based on checkpoint status.
				Name:         "ship/cab-review",
				ModelTier:    ModelTierHeavy,
				KBCheckpoint: "ship",
				KBFamily:     "security",
				KBTags:       []string{"security", "compliance", "observability", "rollback", "deployment"},
				Roles:        []RoleID{RoleOps, RoleSec, RoleCPO},
				DebateRounds: 1,
				Steering:     "review-dab",
				HookGate:     "arch-file-lint",
			},
		},

		// ── qa-verify ─────────────────────────────────────────────────────────
		// Roles: QE + BA + Sec + Ops + CPO + PO (all 6 contribute manual test plan sections)
		// Goal: spec-code gap (det) → automated coverage → 6-role manual plan → gates
		"qa-verify": {
			{
				// ALWAYS runs first — zero tokens. Blocks on incomplete tasks, untested
				// authz roles, and missing event test assertions.
				Name:          "qa-verify/spec-code-gap",
				Deterministic: true,
				HookGate:      "spec-code-alignment-gate",
			},
			{
				// QE verifies automated test suite covers the full test pyramid.
				// KB tags deliberately broad — QE is not just unit tests.
				Name:         "qa-verify/automated-coverage",
				ModelTier:    ModelTierFast,
				KBCheckpoint: "qa-verify",
				KBFamily:     "",
				KBTags: []string{
					"test-design", "quality-gate", "coverage", "tdd",
					"integration-testing", "e2e", "regression",
					"contract-testing", "test-pyramid", "smoke-testing",
				},
				Roles:    []RoleID{RoleQE},
				Steering: "qa-completeness",
			},
			{
				// 6-role manual test plan: each role contributes their dedicated section.
				// Round 1 — independent sections; Round 2 — cross-role gap challenge.
				// Output: .forge/specs/<slug>/manual-test-plan.md
				Name:         "qa-verify/manual-test-plan",
				ModelTier:    ModelTierHeavy,
				KBCheckpoint: "qa-verify",
				KBFamily:     "",
				KBTags: []string{
					"exploratory-testing", "acceptance-testing", "manual-testing",
					"security", "compliance", "performance-testing", "audit-logging",
				},
				Roles:        []RoleID{RolePO, RoleBA, RoleQE, RoleSec, RoleOps, RoleCPO},
				DebateRounds: 2,
				Steering:     "manual-test-plan",
				HookGate:     "manual-test-plan-gate",
			},
			{
				Name:          "qa-verify/coverage-gate",
				Deterministic: true,
				HookGate:      "qa-coverage-gate",
			},
		},
	}
}

// PhasesForCheckpoint returns the sub-workflow phases for the given checkpoint name.
// Returns nil when the checkpoint has no sub-workflow defined.
func PhasesForCheckpoint(checkpoint string) []Phase {
	return defaultSubWorkflows()[checkpoint]
}
