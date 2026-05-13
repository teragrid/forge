// Copyright 2024 The Forge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// roles.go — role-based self-debate engine for forge ship --yolo.
//
// Architecture
// ────────────
// When --yolo is set, forge ship has no human gate between checkpoints.
// Instead, a panel of eight role personas (PO, BA, SA, DL, QE, Sec, Ops, CPO)
// independently reviews each checkpoint deliverable over three rounds:
//
//	Round 1 — Independent review: each role raises concerns from its own lens.
//	Round 2 — Cross-challenge: roles respond to each other's concerns;
//	          minor concerns are resolved, cross-role synthesis items added.
//	Round 3 — Synthesis: remaining major concerns become named improvements;
//	          a polished summary is produced.
//
// In MVP (dry-run), all concerns are capped at SeverityMajor and consensus is
// always reached. In M1, real LLM calls drive each role using the instruction
// files from .forge/instructions/roles/; blocking concerns can halt the pipeline.
//
// The concern catalog in this file is the single source of truth for what each
// role checks in each deliverable type. Adding a new deliverable type only
// requires adding an entry to the relevant catalog map.
package cmdship

import (
	"fmt"
	"strings"
)

// ── Severity ──────────────────────────────────────────────────────────────────

// ConcernSeverity classifies how critical a debate concern is.
type ConcernSeverity string

const (
	// SeverityBlocking must be resolved before the pipeline continues (M1 only).
	SeverityBlocking ConcernSeverity = "blocking"
	// SeverityMajor should be addressed; surfaced prominently in output.
	SeverityMajor ConcernSeverity = "major"
	// SeverityMinor is an improvement note; does not block delivery.
	SeverityMinor ConcernSeverity = "minor"
)

// ── Role ─────────────────────────────────────────────────────────────────────

// RoleID is the canonical identifier for a built-in reviewer persona.
type RoleID string

const (
	RolePO  RoleID = "po"  // Product Owner
	RoleBA  RoleID = "ba"  // Business Analyst
	RoleSA  RoleID = "sa"  // Solution Architect
	RoleDL  RoleID = "dl"  // Delivery Lead / Tech Lead
	RoleQE  RoleID = "qe"  // Quality Engineer
	RoleSec RoleID = "sec" // Security Reviewer
	RoleOps RoleID = "ops" // DevOps / SRE
	RoleCPO RoleID = "cpo" // Compliance & Privacy Officer
)

// Role is a stakeholder persona that reviews deliverables during self-debate.
type Role struct {
	ID         RoleID   `json:"id"`
	Name       string   `json:"name"`
	Hat        string   `json:"hat"`         // review lens / Six-Hats metaphor
	FocusAreas []string `json:"focus_areas"` // primary review concerns
}

// DefaultRoles returns the eight built-in reviewer personas used by SelfDebate.
// In M1, these personas are backed by the instruction files in
// .forge/instructions/roles/ and make real LLM calls.
func DefaultRoles() []Role {
	return []Role{
		{
			ID:   RolePO,
			Name: "Product Owner",
			Hat:  "value",
			FocusAreas: []string{
				"user stories & acceptance criteria",
				"business value & ROI",
				"priority & MVP scope",
				"stakeholder alignment",
			},
		},
		{
			ID:   RoleBA,
			Name: "Business Analyst",
			Hat:  "rules",
			FocusAreas: []string{
				"business rules & constraints",
				"edge cases & error scenarios",
				"data definitions & validation rules",
				"process flows & state transitions",
			},
		},
		{
			ID:   RoleSA,
			Name: "Solution Architect",
			Hat:  "structure",
			FocusAreas: []string{
				"architectural fit & pattern conformance",
				"integration contracts & API schemas",
				"non-functional requirements (latency, throughput, availability)",
				"ADR triggers & tech-debt management",
			},
		},
		{
			ID:   RoleDL,
			Name: "Delivery Lead",
			Hat:  "execution",
			FocusAreas: []string{
				"implementation complexity & estimates",
				"dependency mapping & critical path",
				"risk identification & mitigation",
				"definition of done",
			},
		},
		{
			ID:   RoleQE,
			Name: "Quality Engineer",
			Hat:  "quality",
			FocusAreas: []string{
				"testability & observability",
				"test pyramid coverage plan",
				"regression & mutation risk",
				"quality gates & exit criteria",
			},
		},
		{
			ID:   RoleSec,
			Name: "Security Reviewer",
			Hat:  "threat",
			FocusAreas: []string{
				"threat surface & OWASP Top 10",
				"auth/authz boundaries",
				"data sensitivity & PII handling",
				"secret & key management",
			},
		},
		{
			ID:   RoleOps,
			Name: "DevOps / SRE",
			Hat:  "reliability",
			FocusAreas: []string{
				"deployment readiness & CI/CD configuration",
				"observability: health checks, metrics, tracing",
				"SLO/error-budget impact",
				"rollback procedures & zero-downtime migrations",
			},
		},
		{
			ID:   RoleCPO,
			Name: "Compliance & Privacy Officer",
			Hat:  "compliance",
			FocusAreas: []string{
				"PII classification & data residency",
				"regulatory frameworks (GDPR, HIPAA, SOC 2, PCI-DSS)",
				"consent management & right-to-erasure",
				"audit completeness & evidence-pack generation",
			},
		},
	}
}

// ── Debate types ──────────────────────────────────────────────────────────────

// DebateConcern is a single observation raised by a Role during review.
type DebateConcern struct {
	Role       RoleID          `json:"role"`
	Severity   ConcernSeverity `json:"severity"`
	Area       string          `json:"area"`
	Content    string          `json:"content"`
	Suggestion string          `json:"suggestion,omitempty"`
	Resolved   bool            `json:"resolved,omitempty"`
}

// DebateRound is one complete review pass where all participating roles weigh in.
type DebateRound struct {
	Round    int             `json:"round"`
	Concerns []DebateConcern `json:"concerns"`
	Summary  string          `json:"summary"`
}

// DebateResult is the outcome of a full multi-round self-debate session for one
// checkpoint deliverable.
type DebateResult struct {
	Deliverable       string          `json:"deliverable"`
	Roles             []RoleID        `json:"roles"`
	Rounds            []DebateRound   `json:"rounds"`
	Consensus         bool            `json:"consensus"`
	RemainingConcerns []DebateConcern `json:"remaining_concerns,omitempty"`
	Improvements      []string        `json:"improvements"`
	PolishedSummary   string          `json:"polished_summary"`
}

// DebateOptions controls a single SelfDebate session.
type DebateOptions struct {
	// Deliverable is the checkpoint name: "spec", "breakdown", "code", "verify".
	// Set automatically by RunWithOptions for each checkpoint.
	Deliverable string
	// Feature is the feature name or description; included in the polished summary.
	Feature string
	// Roles lists the review personas (nil = DefaultRoles()).
	Roles []Role
	// MaxRounds caps the debate (0 → default of 3; MVP always runs ≤ 3 rounds).
	MaxRounds int
	// DryRun caps all concerns at SeverityMajor so consensus is always reached.
	DryRun bool
}

// RunOptions controls a full RunWithOptions execution.
type RunOptions struct {
	Root        string
	Description string
	Names       []string       // nil = all five checkpoints
	Gate        Gate           // nil = YOLO (no prompts)
	DebateOpts  *DebateOptions // nil = no self-debate; set when --yolo is used
	// LLMPipe is an optional pre-built pipe for testing. nil = auto-detect via
	// llmprovider.Detect (uses ANTHROPIC_API_KEY / OPENAI_API_KEY).
	LLMPipe *LLMPipe
	// CreatePR, when true, appends a PR-creation checkpoint after verify.
	// Only effective for full-pipeline runs (Names == nil). Triggered by --pr.
	CreatePR bool
}

// ── Concern catalog ───────────────────────────────────────────────────────────

// ct is a concern template entry in the catalog.
type ct struct {
	Area       string
	Content    string
	Suggestion string
	Sev        ConcernSeverity
}

// specCatalog contains the MVP stub concerns each role raises when reviewing a spec.
var specCatalog = map[RoleID][]ct{
	RolePO: {
		{
			"user stories",
			"Acceptance criteria not written in Given/When/Then format",
			"Rewrite ACs as GWT scenarios covering the happy path and at least two unhappy paths",
			SeverityMajor,
		},
		{
			"business value",
			"No explicit business-value statement in the spec",
			"Add: \"As a <persona> I want <goal> so that <outcome>\"",
			SeverityMajor,
		},
		{
			"scope boundary",
			"In-scope vs. out-of-scope not explicitly stated",
			"Add a scope table: one column for included items, one for explicitly excluded",
			SeverityMinor,
		},
	},
	RoleBA: {
		{
			"edge cases",
			"Concurrent update scenario not specified",
			"Define conflict-resolution strategy: last-write-wins, optimistic lock, or queue",
			SeverityMajor,
		},
		{
			"validation rules",
			"Input validation constraints not documented",
			"Enumerate field constraints: type, max length, format, nullability, allowed values",
			SeverityMajor,
		},
		{
			"state transitions",
			"State-machine transitions not documented",
			"Add a state diagram or transition table (e.g. draft→pending→approved→rejected)",
			SeverityMinor,
		},
	},
	RoleSA: {
		{
			"NFRs",
			"Non-functional requirements (latency, throughput, availability) absent",
			"Add SLO targets: p99 latency < X ms, throughput > Y RPS, availability ≥ 99.9%",
			SeverityMajor,
		},
		{
			"integration contract",
			"Upstream/downstream integration contract not defined",
			"Specify API contract: endpoint, request/response schema, error codes, versioning",
			SeverityMajor,
		},
		{
			"ADR trigger",
			"No ADR flagged despite a potential architectural decision",
			"Evaluate whether this feature requires a new ADR; create one if yes",
			SeverityMinor,
		},
	},
	RoleDL: {
		{
			"complexity",
			"Implementation complexity not estimated",
			"Add T-shirt sizing (XS/S/M/L/XL) or story-point estimate with rationale",
			SeverityMinor,
		},
		{
			"definition of done",
			"Definition of Done not specified for this feature",
			"Add DoD: all tests green, docs updated, security review done, feature flag configured",
			SeverityMajor,
		},
		{
			"dependencies",
			"External dependencies not mapped",
			"List all services and libraries this feature depends on with their owning teams",
			SeverityMinor,
		},
	},
	RoleQE: {
		{
			"testability",
			"Testability of external dependencies not assessed",
			"Identify which dependencies need mocks or stubs; add mock strategy to test plan",
			SeverityMajor,
		},
		{
			"quality gates",
			"Quality gates (coverage threshold, mutation score) not defined",
			"Specify: line coverage ≥ 80%, mutation score ≥ 60%, no critical lint errors",
			SeverityMajor,
		},
		{
			"test pyramid",
			"Test pyramid allocation not planned",
			"Allocate test budget: unit/integration/e2e ratio (e.g. 70/20/10)",
			SeverityMinor,
		},
	},
	RoleSec: {
		{
			"auth boundary",
			"Authentication/authorisation boundary not drawn",
			"Add auth/authz section: who may call this, required scopes/roles, token validation",
			SeverityMajor,
		},
		{
			"PII classification",
			"Personally identifiable data not classified",
			"Label fields as PII/non-PII; specify encryption-at-rest and in-transit requirements",
			SeverityMajor,
		},
		{
			"OWASP checklist",
			"OWASP Top 10 (A01–A10) not assessed for this feature",
			"Run through A01–A10; mark N/A with rationale; address applicable items",
			SeverityMinor,
		},
	},
	RoleOps: {
		{
			"deployment impact",
			"Deployment impact not assessed (migration, infra change, config update)",
			"Add deployment section: list required migration, config change, or infra update",
			SeverityMajor,
		},
		{
			"observability requirements",
			"Observability requirements (metrics, logs, traces) not specified in spec",
			"Define which new metrics, log events, or traces this feature introduces",
			SeverityMajor,
		},
		{
			"rollback criteria",
			"Rollback criteria and procedure not defined",
			"Add rollback section: trigger conditions, rollback command, expected recovery time",
			SeverityMinor,
		},
	},
	RoleCPO: {
		{
			"PII inventory",
			"PII inventory not present in spec",
			"Enumerate all PII/sensitive fields; classify each (name, email, health, financial)",
			SeverityMajor,
		},
		{
			"regulatory scope",
			"Applicable compliance frameworks not identified",
			"State which frameworks apply (GDPR, HIPAA, SOC 2, PCI-DSS) and why",
			SeverityMajor,
		},
		{
			"right-to-erasure",
			"Right-to-erasure path not defined for new PII fields",
			"Specify how forge audit erase cascades through new PII fields",
			SeverityMinor,
		},
	},
}

// breakdownCatalog contains stub concerns each role raises for a breakdown checkpoint.
var breakdownCatalog = map[RoleID][]ct{
	RolePO: {
		{
			"story traceability",
			"Tasks not traceable to user stories",
			"Add a 'Addresses: US-XX' tag to each task",
			SeverityMinor,
		},
		{
			"value ordering",
			"Tasks not prioritized by customer value",
			"Reorder breakdown so highest-value tasks appear first",
			SeverityMajor,
		},
	},
	RoleBA: {
		{
			"edge-case tasks",
			"Edge-case handling tasks absent from breakdown",
			"Add explicit tasks for: concurrent update, invalid input, timeout, partial failure",
			SeverityMajor,
		},
		{
			"rollback task",
			"No rollback or compensating-transaction task in breakdown",
			"Add task: implement rollback/undo if feature mutates persistent state",
			SeverityMinor,
		},
	},
	RoleSA: {
		{
			"ADR task",
			"No task to write or update an ADR",
			"Add task: write ADR-NNN for the architectural decision this feature introduces",
			SeverityMajor,
		},
		{
			"migration task",
			"Database/schema migration not in breakdown",
			"Add tasks: write migration, test migration, test rollback",
			SeverityMinor,
		},
	},
	RoleDL: {
		{
			"time-boxing",
			"Tasks are unbounded — no effort estimate",
			"Add max-effort estimate per task (e.g. ≤ 4 h or ≤ 1 d)",
			SeverityMinor,
		},
		{
			"blockers",
			"Tasks with external blockers not flagged",
			"Mark tasks that depend on external teams or unavailable environments",
			SeverityMajor,
		},
	},
	RoleQE: {
		{
			"test tasks",
			"Test tasks not linked to each implementation task",
			"Add 'Write tests for X' task adjacent to each 'Implement X' task",
			SeverityMajor,
		},
		{
			"automation task",
			"No task to wire new tests into CI",
			"Add task: add test suite to CI pipeline; define pass/fail threshold",
			SeverityMinor,
		},
	},
	RoleSec: {
		{
			"security review task",
			"No security review task scheduled before merge",
			"Add task: schedule security review as a merge prerequisite",
			SeverityMajor,
		},
		{
			"secrets task",
			"No task to provision or rotate secrets if new credentials are needed",
			"Add task: provision/rotate secrets via secrets manager",
			SeverityMinor,
		},
	},
	RoleOps: {
		{
			"deployment task",
			"No deployment or infra-update task in the breakdown",
			"Add task: update CI/CD workflow, environment config, or migration runner",
			SeverityMajor,
		},
		{
			"observability task",
			"No observability task (metrics, dashboard, alerts) in the breakdown",
			"Add task: add/update metrics, alert rules, and dashboards for this feature",
			SeverityMinor,
		},
	},
	RoleCPO: {
		{
			"PII tagging task",
			"No PII-tagging task for new sensitive fields in the breakdown",
			"Add task: annotate all new PII fields with classification tags",
			SeverityMajor,
		},
		{
			"erasure handler task",
			"No right-to-erasure handler task despite new PII fields being introduced",
			"Add task: implement and test forge audit erase cascade for new PII fields",
			SeverityMinor,
		},
	},
}

// codeCatalog contains stub concerns each role raises for a code checkpoint.
var codeCatalog = map[RoleID][]ct{
	RolePO: {
		{
			"feature flag",
			"Feature flag not considered for gradual rollout",
			"Wrap new behaviour in a feature flag for controlled release",
			SeverityMinor,
		},
	},
	RoleBA: {
		{
			"business rules in service",
			"Verify that business rules live in the service layer, not DB triggers",
			"Move business logic to the service layer; avoid encoding rules in DB triggers",
			SeverityMinor,
		},
	},
	RoleSA: {
		{
			"pattern conformance",
			"Code may deviate from established architectural patterns",
			"Review against ADR and module structure conventions in .forge/instructions/",
			SeverityMinor,
		},
	},
	RoleDL: {
		{
			"TODO debt",
			"TODOs/FIXMEs without a tracking issue create invisible debt",
			"Convert all TODO/FIXME to tracked issues before merge",
			SeverityMinor,
		},
	},
	RoleQE: {
		{
			"branch coverage",
			"Error-path branches may lack test coverage",
			"Add tests for each error return path and nil/empty input combination",
			SeverityMajor,
		},
		{
			"missing error tests",
			"Happy-path tests present but negative-case tests absent",
			"Add negative tests: invalid input, unauthorized, service-unavailable",
			SeverityMajor,
		},
	},
	RoleSec: {
		{
			"hardcoded secrets",
			"Potential for hardcoded credentials or API keys in source",
			"Run `forge scan secrets`; move any secrets to the secrets manager",
			SeverityMajor,
		},
		{
			"injection surface",
			"SQL/command injection surface not reviewed",
			"Audit all external-input paths; use parameterized queries and safe APIs",
			SeverityMajor,
		},
	},
	RoleOps: {
		{
			"health check",
			"New external dependency not covered by health-check endpoint",
			"Update /health or /ready endpoint to include the new dependency's reachability check",
			SeverityMajor,
		},
	},
	RoleCPO: {
		{
			"consent before collect",
			"Consent record must be written before PII is stored",
			"Verify consent is captured in the same transaction or prior to any PII write",
			SeverityMajor,
		},
	},
}

// verifyCatalog contains stub concerns each role raises for a verify checkpoint.
var verifyCatalog = map[RoleID][]ct{
	RolePO: {
		{
			"changelog",
			"CHANGELOG.md not updated with this feature",
			"Add entry under Unreleased: [feature] brief description",
			SeverityMinor,
		},
	},
	RoleBA: {
		{
			"AC sign-off",
			"Acceptance criteria not all marked as done",
			"Walk through each AC; mark done or open a bug for any incomplete ones",
			SeverityMinor,
		},
	},
	RoleSA: {
		{
			"ADR up-to-date",
			"ADRs may be out of date with the implementation",
			"Review ADRs touched by this feature; update status if superseded",
			SeverityMinor,
		},
	},
	RoleDL: {
		{
			"CI clean",
			"All CI checks should be green before verify passes",
			"Investigate and fix any failing CI checks before merging",
			SeverityMinor,
		},
	},
	RoleQE: {
		{
			"all tests green",
			"Verify all tests pass before ship",
			"Run `forge test all`; fix any failures before proceeding",
			SeverityMinor,
		},
	},
	RoleSec: {
		{
			"secret scan clean",
			"Secret scan should be clean before ship",
			"Run `forge scan secrets`; remediate any findings",
			SeverityMinor,
		},
	},
	RoleOps: {
		{
			"reliability scan",
			"Reliability scan should be clean before ship",
			"Run `forge scan reliability`; fix idempotency, outbox, and circuit-breaker gaps",
			SeverityMinor,
		},
	},
	RoleCPO: {
		{
			"compliance scan",
			"Compliance scan should be clean before ship",
			"Run `forge scan compliance`; remediate untagged PII, missing consent records",
			SeverityMinor,
		},
	},
}

// genericCatalog is the fallback for unknown deliverable types.
var genericCatalog = map[RoleID][]ct{
	RolePO:  {{"quality", "Review for business value alignment", "Validate against user stories", SeverityMinor}},
	RoleBA:  {{"quality", "Review for completeness", "Confirm edge cases are covered", SeverityMinor}},
	RoleSA:  {{"quality", "Review for architectural fit", "Check pattern conformance", SeverityMinor}},
	RoleDL:  {{"quality", "Review for implementation risk", "Validate estimates", SeverityMinor}},
	RoleQE:  {{"quality", "Review for testability", "Confirm quality gates are set", SeverityMinor}},
	RoleSec: {{"quality", "Review for security posture", "Run OWASP Top 10 checklist", SeverityMinor}},
	RoleOps: {{"quality", "Review for operational readiness", "Confirm deployment and rollback procedures", SeverityMinor}},
	RoleCPO: {{"quality", "Review for compliance posture", "Confirm PII tagging and regulatory scope", SeverityMinor}},
}

// ── Debate engine ─────────────────────────────────────────────────────────────

// SelfDebate runs a multi-round structured role-based review of a checkpoint
// deliverable. It is called automatically by RunWithOptions when DebateOpts is set
// (i.e. when --yolo is active on the full pipeline).
//
// MVP behaviour (DryRun=true):
//   - All concerns are capped at SeverityMajor.
//   - Consensus is always reached after maxRounds (informational output).
//   - Round 1: each role independently raises concerns.
//   - Round 2: minor concerns resolved; cross-role synthesis items added.
//   - Round 3: remaining major concerns become named improvements; consensus declared.
//
// M1 behaviour (DryRun=false, LLM available):
//   - Each role makes a real LLM call with its instruction file as the system prompt.
//   - Blocking concerns genuinely block the pipeline until resolved.
func SelfDebate(opts DebateOptions) *DebateResult {
	roles := opts.Roles
	if len(roles) == 0 {
		roles = DefaultRoles()
	}
	maxRounds := opts.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 3
	}
	if maxRounds > 3 { // MVP cap
		maxRounds = 3
	}

	result := &DebateResult{
		Deliverable: opts.Deliverable,
		Roles:       roleIDs(roles),
	}

	// ── Round 1: independent review ─────────────────────────────────────────
	round1 := DebateRound{Round: 1}
	for _, r := range roles {
		for _, tmpl := range catalogFor(r.ID, opts.Deliverable) {
			sev := tmpl.Sev
			if opts.DryRun && sev == SeverityBlocking {
				sev = SeverityMajor // downgrade blocking to major in dry-run
			}
			round1.Concerns = append(round1.Concerns, DebateConcern{
				Role:       r.ID,
				Severity:   sev,
				Area:       tmpl.Area,
				Content:    tmpl.Content,
				Suggestion: tmpl.Suggestion,
			})
		}
	}
	round1.Summary = fmt.Sprintf(
		"%d roles raised %d concern(s) across %d area(s)",
		len(roles), len(round1.Concerns), uniqueAreas(round1.Concerns),
	)
	result.Rounds = append(result.Rounds, round1)

	if maxRounds < 2 {
		result.Consensus = true
		result.PolishedSummary = polishedSummary(opts.Deliverable, opts.Feature, 0, len(roles))
		return result
	}

	// ── Round 2: cross-challenge ─────────────────────────────────────────────
	round2 := DebateRound{Round: 2}
	resolvedMinor := 0
	// Copy concerns from round 1; resolve all minor ones.
	updated := make([]DebateConcern, len(round1.Concerns))
	copy(updated, round1.Concerns)
	for i := range updated {
		if updated[i].Severity == SeverityMinor {
			updated[i].Resolved = true
			resolvedMinor++
		}
	}
	// Add cross-role synthesis items.
	cross := crossItems(opts.Deliverable)
	updated = append(updated, cross...)
	round2.Concerns = updated
	round2.Summary = fmt.Sprintf(
		"Cross-challenge: %d minor concern(s) resolved; %d cross-role synthesis item(s) added",
		resolvedMinor, len(cross),
	)
	result.Rounds = append(result.Rounds, round2)

	if maxRounds < 3 {
		result.Consensus = true
		result.PolishedSummary = polishedSummary(opts.Deliverable, opts.Feature, 0, len(roles))
		return result
	}

	// ── Round 3: synthesis ───────────────────────────────────────────────────
	round3 := DebateRound{Round: 3}
	var improvements []string
	finalConcerns := make([]DebateConcern, len(round2.Concerns))
	copy(finalConcerns, round2.Concerns)
	for i := range finalConcerns {
		c := &finalConcerns[i]
		if !c.Resolved && c.Severity == SeverityMajor {
			improvements = append(improvements,
				fmt.Sprintf("[%s/%s] %s", c.Role, c.Area, c.Suggestion))
			c.Resolved = true
		}
	}
	round3.Concerns = finalConcerns
	round3.Summary = fmt.Sprintf(
		"Synthesis: %d improvement(s) captured; all concerns addressed for this round",
		len(improvements),
	)
	result.Rounds = append(result.Rounds, round3)
	result.Improvements = improvements

	// In MVP dry-run, no blocking concerns remain → consensus always reached.
	result.Consensus = true
	result.PolishedSummary = polishedSummary(opts.Deliverable, opts.Feature, len(improvements), len(roles))
	return result
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// roleIDs returns the RoleID of each Role in the slice.
func roleIDs(roles []Role) []RoleID {
	ids := make([]RoleID, len(roles))
	for i, r := range roles {
		ids[i] = r.ID
	}
	return ids
}

// catalogFor returns the concern templates for (role, deliverable).
func catalogFor(id RoleID, deliverable string) []ct {
	var m map[RoleID][]ct
	switch strings.ToLower(deliverable) {
	case "spec":
		m = specCatalog
	case "breakdown":
		m = breakdownCatalog
	case "code":
		m = codeCatalog
	case "verify":
		m = verifyCatalog
	default:
		m = genericCatalog
	}
	return m[id]
}

// crossItems returns cross-role synthesis concerns that only emerge from role
// interaction (added in Round 2). These are pre-resolved as they represent
// insights already incorporated into the polished summary.
func crossItems(deliverable string) []DebateConcern {
	switch strings.ToLower(deliverable) {
	case "spec":
		return []DebateConcern{
			{
				Role: RoleSA, Severity: SeverityMinor,
				Area:       "cross: PO↔SA",
				Content:    "Acceptance criteria reference UI behaviour but no API contract is defined",
				Suggestion: "Align PO acceptance criteria with the SA API contract to prevent mismatch",
				Resolved:   true,
			},
			{
				Role: RoleQE, Severity: SeverityMinor,
				Area:       "cross: BA↔QE",
				Content:    "BA edge cases (concurrent update, timeout) not reflected in test plan",
				Suggestion: "Map each BA edge case to a corresponding test scenario",
				Resolved:   true,
			},
		}
	case "breakdown":
		return []DebateConcern{
			{
				Role: RoleDL, Severity: SeverityMinor,
				Area:       "cross: QE↔DL",
				Content:    "Test tasks listed but not on the critical path — risk of de-prioritisation",
				Suggestion: "Move test tasks adjacent to (not after) each implementation task",
				Resolved:   true,
			},
		}
	default:
		return nil
	}
}

// uniqueAreas counts distinct concern areas.
func uniqueAreas(concerns []DebateConcern) int {
	seen := map[string]struct{}{}
	for _, c := range concerns {
		seen[c.Area] = struct{}{}
	}
	return len(seen)
}

// polishedSummary builds the human-readable debate outcome sentence.
func polishedSummary(deliverable, feature string, improvements, roles int) string {
	subject := deliverable
	if feature != "" {
		subject = fmt.Sprintf("%s (%s)", deliverable, feature)
	}
	return fmt.Sprintf(
		"%d-role panel reviewed %s over 3 rounds; %d improvement(s) captured. "+
			"Apply the improvements list before M1 execution to maximise deliverable quality.",
		roles, subject, improvements,
	)
}
