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

// arch.go — Checkpoint 2: multi-role architecture debate → ADR-format document.
//
// checkArch reads the feature spec produced by checkSpec and runs a 3-round
// debate with six architecture-focused roles (Systems, Security, Data, API,
// Performance, Platform). The polished output is an Architecture Decision
// Record (ADR) written to .forge/specs/<slug>/arch.md.
//
// Role personas used (separate from the 8 general roles in roles.go):
//   RoleSysArch   — component topology, ADR triggers, scalability
//   RoleSecArch   — STRIDE threat model, zero-trust, secret rotation
//   RoleDatArch   — schema migrations, consistency model, versioning
//   RoleAPIDesign — API contracts, versioning strategy, idempotency
//   RolePerfEng   — latency budgets, cache invalidation, throughput
//   RolePlatArch  — deployment topology, observability, DR plan

package cmdship

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// archRoleDebate holds one reviewer role for the parallel architecture debate.
type archRoleDebate struct {
	persona string // LLM persona instruction
	name    string // short label (e.g. "security", "performance")
}

// defaultArchRoles returns the six specialist reviewer personas used in
// the parallel architecture debate. Each role focuses on a different
// cross-cutting concern and may raise objections or suggest improvements.
func defaultArchRoles() []archRoleDebate {
	return []archRoleDebate{
		{name: "systems", persona: "You are a Senior Systems Architect. Evaluate component topology, scalability, and ADR quality."},
		{name: "security", persona: "You are a Security Architect. Apply STRIDE threat modelling and zero-trust review."},
		{name: "data", persona: "You are a Data Architect. Review schema migrations, consistency model, and versioning strategy."},
		{name: "api", persona: "You are an API Design Expert. Evaluate API contracts, versioning, and idempotency guarantees."},
		{name: "performance", persona: "You are a Performance Engineer. Assess latency budgets, caching strategy, and throughput targets."},
		{name: "platform", persona: "You are a Platform Engineer. Review deployment topology, observability, and disaster recovery."},
	}
}

// runParallelArchDebate concurrently invokes all arch reviewer roles and
// collects their concerns. Results are appended to the arch document as a
// "## Reviewer Concerns" section. A nil pipe is a no-op.
func runParallelArchDebate(pipe *LLMPipe, description, archDoc string, maxTokens int) string {
	if pipe == nil {
		return ""
	}
	roles := defaultArchRoles()
	type result struct {
		name    string
		concern string
	}
	results := make([]result, len(roles))
	var wg sync.WaitGroup

	for i, role := range roles {
		wg.Add(1)
		go func(idx int, r archRoleDebate) {
			defer wg.Done()
			concern, err := pipe.InvokeDebateRound(
				"arch-parallel-debate",
				description,
				r.persona,
				archDoc,
				"",
				maxTokens,
			)
			if err != nil || strings.TrimSpace(concern) == "" {
				concern = "(no concerns raised)"
			}
			results[idx] = result{name: r.name, concern: concern}
		}(i, role)
	}
	wg.Wait()

	var sb strings.Builder
	sb.WriteString("\n\n## Reviewer Concerns (parallel debate)\n\n")
	for _, r := range results {
		sb.WriteString("### ")
		sb.WriteString(r.name)
		sb.WriteString("\n\n")
		sb.WriteString(strings.TrimSpace(r.concern))
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// checkArch generates or validates the architecture document for a feature.
// It is Checkpoint 2 in the 6-checkpoint pipeline, running immediately after
// checkSpec and before checkTest.
//
// specName, when non-empty, overrides the slug derived from description and
// lets callers target a named spec directory (e.g. --name login).
//
// Behaviour:
//   - If arch.md already exists: returns "ok" immediately (idempotent).
//   - If spec.md is missing: returns "warning" with a hint to run spec first.
//   - With an LLMPipe: generates full ADR then runs parallel role debate.
//   - Without an LLMPipe: writes a structured stub ADR to arch.md.
func checkArch(root, description, specName string, pipe *LLMPipe) Checkpoint {
	cp := Checkpoint{Name: "Arch"}

	if description == "" && specName == "" {
		cp.Status = "warning"
		if pipe != nil {
			cp.Detail = fmt.Sprintf(
				"no --description; pass --description <feature> to generate arch document via %s",
				pipe.ProviderName())
		} else {
			cp.Detail = "no --description; pass --description <feature> to generate architecture document"
		}
		return cp
	}

	// Determine slug: --name/-n takes priority over the slug derived from description.
	slug := specName
	if slug == "" {
		slug = slugify(description)
	}
	// When only --name is given (no description), use it as LLM feature context.
	if description == "" {
		description = specName
	}
	archFile := filepath.Join(root, ".forge", "specs", slug, "arch.md")
	openapiFile := filepath.Join(root, ".forge", "specs", slug, "openapi.yaml")

	// Idempotent: arch document already present.
	if _, err := os.Stat(archFile); err == nil {
		cp.Status = "ok"
		cp.Detail = fmt.Sprintf("arch document found: .forge/specs/%s/arch.md", slug)
		// Back-fill openapi.yaml stub if missing (e.g. feature pre-dates this change).
		if _, err := os.Stat(openapiFile); err != nil {
			_ = os.WriteFile(openapiFile, []byte(openapiStub(description)), 0o600)
		}
		return cp
	}

	// Load spec context (no spec is a warning, not a failure — allow pipeline to continue).
	specMD := ""
	specPath := filepath.Join(root, ".forge", "specs", slug, "spec.md")
	if data, err := os.ReadFile(specPath); err == nil {
		specMD = string(data)
	} else {
		cp.Status = "warning"
		cp.Detail = fmt.Sprintf(
			"no spec found at .forge/specs/%s/spec.md — run forge ship spec first, then re-run forge ship arch",
			slug)
		return cp
	}

	// Ensure the spec directory exists before writing arch.md.
	_ = os.MkdirAll(filepath.Join(root, ".forge", "specs", slug), 0o755)

	// G-009/J3: forward the same deterministic workspace-context summary
	// checkSpec already collects (tech stack, conventions, project overview)
	// into the arch prompt, so it can't invent infrastructure (Redis,
	// DataDog, NextAuth.js, etc.) absent from the project's actual
	// dependencies — the exact hallucination observed dogfooding this on
	// ai-marketing-platfrom.
	wsCtx := collectWorkspaceContext(root, slug)
	wsSection := ""
	if wsCtx.Content != "" {
		wsSection = "\n\n## Workspace Context\n" + wsCtx.Content
	}

	// Generate arch document via LLM or write a structured stub.
	archContent := ""
	openapiContent := ""
	if pipe != nil {
		archGenSystem := "You are a Solution Architect performing an architecture review. " +
			"Given the feature specification and the project's real workspace context (tech stack, " +
			"existing dependencies, structure), produce a concise Architecture Decision Record (ADR) in " +
			"Markdown. Ground every component and dependency you name in the actual workspace context " +
			"provided — never invent infrastructure, services, or libraries that are not evidenced there. " +
			"Structure the document with these sections: " +
			"1) Component Topology — list components, their boundaries, and relationships; " +
			"2) API Contracts — reference the openapi.yaml you will produce below; state which API style is used; " +
			"3) Data Model & Consistency — data entities, migration strategy, consistency model; " +
			"4) Non-Functional Requirements — p99 latency, throughput, availability SLOs; " +
			"5) Security Threat Model — STRIDE threats, mitigations, auth/authz boundaries; " +
			"6) Deployment & Observability — topology, health checks, metrics, tracing, DR plan. " +
			"End with a concise ADR summary: Status, Context, Decision, Consequences. " +
			"After the ADR, append a fenced ```yaml block containing a valid OpenAPI 3.1.0 contract " +
			"for all API endpoints introduced by this feature. " +
			"API style: if the feature is backed by Supabase, define each operation as " +
			"POST /rest/v1/rpc/{function_name} with a requestBody of type object and the PostgreSQL " +
			"function parameters as properties; include the Supabase anon/service-role security scheme. " +
			"For standard REST features, use resource-oriented paths (e.g. POST /api/v1/{resource}). " +
			"The block will be extracted and saved as openapi.yaml automatically."

		generateFn := func() (string, bool, error) {
			return pipe.InvokeWithKnowledgeChecked(
				"ship:arch:generate", "",
				archGenSystem,
				fmt.Sprintf("Feature: %s%s\n\nSpec:\n%s", description, wsSection, specMD),
				// 6000: the prompt above asks for 6 full ADR sections *plus* a
				// complete OpenAPI 3.1 contract. 3000 was reliably too small for
				// that shape — every real run truncated, and the retry in
				// generateWithValidation used the same budget so it truncated
				// again, making "truncated after retry" near-deterministic
				// rather than a rare flake (root-caused 2026-07-19; see also
				// AnthropicAdapter.continueOnMaxTokens, which softens this by
				// continuing a cut-off response instead of discarding it —
				// but it replays at the *same* budget, it does not raise it,
				// so the base budget here still has to be sized correctly).
				6000,
				"arch", "api-design", "dab",
				[]string{"openapi", "architecture", "api-contract", "supabase"},
			)
		}
		// J8/J9: strip conversational preamble and detect truncation before
		// writing; retry once on an incomplete response.
		generated, complete, err := generateWithValidation(generateFn)
		switch {
		case err != nil:
			archContent = archStub(description)
			openapiContent = openapiStub(description)
			cp.Detail = fmt.Sprintf(
				"arch stub created (LLM:%s — %s): .forge/specs/%s/arch.md + openapi.yaml — edit before continuing",
				pipe.ProviderName(), llmErrNote(err), slug)
			// G-011/J7: record LLM failures for future learning context,
			// mirroring the existing test/breakdown/qa-verify wiring.
			appendFailure(root, "arch", description, cp.Detail+" | full: "+llmErrFull(err))
		case generated != "" && complete:
			archContent, openapiContent = extractOpenAPIBlock(generated, description)
			// P1: run parallel role debate and append reviewer concerns.
			debateSuffix := runParallelArchDebate(pipe, description, archContent, 300)
			if debateSuffix != "" {
				archContent += debateSuffix
			}
			archContent = appendUnverifiedPathsWarning(root, archContent)
			cp.Detail = fmt.Sprintf(
				"arch document + openapi.yaml generated by %s (parallel role debate applied): .forge/specs/%s/",
				pipe.ProviderName(), slug)
		case generated != "" && !complete:
			// J9: LLM produced content but it looks truncated/incomplete
			// even after a retry — never silently write a broken file.
			archContent = archStub(description)
			openapiContent = openapiStub(description)
			cp.Detail = fmt.Sprintf(
				"arch generation truncated/incomplete after retry (LLM:%s) — stub created: "+
					".forge/specs/%s/arch.md + openapi.yaml — edit before continuing",
				pipe.ProviderName(), slug)
			appendFailure(root, "arch", description, cp.Detail)
		default:
			archContent = archStub(description)
			openapiContent = openapiStub(description)
			cp.Detail = fmt.Sprintf(
				"arch stub created: .forge/specs/%s/arch.md + openapi.yaml — edit before continuing", slug)
		}
	} else {
		archContent = archStub(description)
		openapiContent = openapiStub(description)
		cp.Detail = fmt.Sprintf(
			"arch stub created: .forge/specs/%s/arch.md + openapi.yaml — edit before continuing "+
				"(run 'forge config set llm.provider <name>' or set ANTHROPIC_API_KEY / OPENAI_API_KEY)",
			slug)
	}

	if err := os.WriteFile(archFile, []byte(archContent), 0o600); err != nil {
		// .forge/ not writable — still ok for dry-run.
		cp.Status = "ok"
		cp.Detail = fmt.Sprintf(
			"description: %q (write .forge/specs/ to persist arch document)", description)
		return cp
	}
	_ = os.WriteFile(openapiFile, []byte(openapiContent), 0o600)

	cp.Status = "ok"
	return cp
}

// extractOpenAPIBlock separates the arch ADR markdown from an embedded OpenAPI
// YAML code block in an LLM response. If no valid OpenAPI block is found,
// the full content becomes arch.md and a stub openapi.yaml is returned.
func extractOpenAPIBlock(content, description string) (archMD, openapiYAML string) {
	// Locate the first ```yaml fenced block that contains "openapi:".
	const fence = "```yaml"
	idx := strings.Index(content, fence)
	for idx >= 0 {
		rest := content[idx+len(fence):]
		endIdx := strings.Index(rest, "```")
		if endIdx < 0 {
			break
		}
		block := strings.TrimSpace(rest[:endIdx])
		if strings.Contains(block, "openapi:") {
			archMD = strings.TrimSpace(content[:idx]) +
				"\n\n> See [openapi.yaml](openapi.yaml) for the full API contract.\n"
			return archMD, block
		}
		// Not an OpenAPI block — keep searching.
		next := strings.Index(content[idx+1:], fence)
		if next < 0 {
			break
		}
		idx = idx + 1 + next
	}
	// No OpenAPI block found — return full content as arch.md and a stub.
	return content, openapiStub(description)
}

// detectAPIStyle returns "supabase-rpc" when the given openapi.yaml content
// uses Supabase PostgREST RPC paths (/rest/v1/rpc/), otherwise "rest".
func detectAPIStyle(openapiContent string) string {
	if strings.Contains(openapiContent, "/rest/v1/rpc/") {
		return "supabase-rpc"
	}
	return "rest"
}

// openapiStub returns a valid OpenAPI 3.1.0 YAML stub that the developer
// fills in as part of the architecture step.
//
// The stub includes both a standard REST path and a commented Supabase RPC
// variant so the developer can choose and delete the one not applicable.
func openapiStub(feature string) string {
	return fmt.Sprintf(`openapi: "3.1.0"
info:
  title: "%s"
  version: "0.0.0-draft"
  description: |
    API contract for %s.
    Edit this file, then re-run `+"`"+`forge ship arch`+"`"+` to validate.
    API style: replace /api/v1/TODO with /rest/v1/rpc/{fn} if using Supabase RPC.
paths:
  /api/v1/TODO:
    post:
      summary: "TODO: describe the primary operation"
      operationId: todoCreate
      tags:
        - TODO
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/TODORequest'
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/TODOResponse'
        "400":
          description: Invalid request payload
        "401":
          description: Authentication required
        "422":
          description: Unprocessable entity
        "500":
          description: Internal server error
components:
  schemas:
    TODORequest:
      type: object
      required: []
      properties: {}
    TODOResponse:
      type: object
      properties:
        id:
          type: string
          format: uuid
`, feature, feature)
}

// archStub returns a minimal but structured Architecture Decision Record stub
// that the developer fills in before the pipeline continues.
func archStub(feature string) string {
	return fmt.Sprintf(`# Architecture: %s

> **Status**: Draft — fill in each section before running forge ship test.

---

## 1. Component Topology

> TODO: Describe components, their boundaries, and relationships.
>
> Example:
> - **API Gateway** — routes requests; enforces auth
> - **Feature Service** — owns business logic; exposes gRPC + REST
> - **Database** — PostgreSQL 15; single writer, read replicas

## 2. API Contracts

> See openapi.yaml in this directory for the full API contract.
> Edit openapi.yaml and re-run "forge ship arch" to validate.
>
> **API style** (choose one and delete the other):
> - **Standard REST** — resource-oriented paths, e.g. POST /api/v1/{resource}
> - **Supabase RPC** — PostgREST function exposure, e.g. POST /rest/v1/rpc/{function_name}
>
> Summary of primary endpoints (auto-populated from openapi.yaml when available):
>
> | Method | Path | Description | Schema |
> |--------|------|-------------|--------|
> | POST | /api/v1/TODO | TODO | TODORequest -> TODOResponse |
> | POST | /rest/v1/rpc/TODO | (Supabase RPC alternative — delete if not using Supabase) | params -> result |

## 3. Data Model & Consistency

> TODO: Describe data entities, migration strategy, and consistency model.

### Entities

- **Entity**: fields, indexes, constraints

### Consistency Model

> TODO: Choose one: strong consistency | eventual consistency (saga) | two-phase commit

### Migration Plan

> TODO: Describe migration steps, rollback script, and CI test coverage.

## 4. Non-Functional Requirements

| NFR          | Target         | Measurement              |
|--------------|----------------|--------------------------|
| p99 latency  | < 200 ms       | Prometheus histogram     |
| Throughput   | > 100 RPS      | Load test (k6)           |
| Availability | ≥ 99.9%%       | SLO burn-rate alert      |

## 5. Security Threat Model

> TODO: Identify STRIDE threats and mitigations.

| Threat | Category | Mitigation |
|--------|----------|------------|
| ... | Spoofing | ... |

### Auth/Authz Boundaries

> TODO: Specify who may call each endpoint, required scopes/roles, and token validation.

## 6. Deployment & Observability

### Deployment Topology

> TODO: Describe replicas, regions, autoscaling policy, and IaC module.

### Observability

| Signal  | Name                   | Description |
|---------|------------------------|-------------|
| Counter | feature_requests_total | ... |
| Histogram | feature_latency_seconds | ... |

### Disaster Recovery

> TODO: Document failover trigger, DNS TTL, and data-replication lag tolerance.

---

## ADR Summary

**Status**: Proposed

**Context**: %s

**Decision**: TODO — record the key architectural decision here.

**Consequences**:
- ✓ TODO: positive consequence
- ✗ TODO: trade-off or risk to mitigate
`, feature, feature)
}
