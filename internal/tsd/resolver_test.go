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

package tsd_test

import (
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/tsd"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func resolve(t *testing.T, yaml string) ([]string, error) {
	t.Helper()
	parsed, err := tsd.Parse(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return tsd.Resolve(parsed)
}

func mustResolve(t *testing.T, yaml string) []string {
	t.Helper()
	modules, err := resolve(t, yaml)
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	return modules
}

func containsModule(modules []string, id string) bool {
	for _, m := range modules {
		if m == id {
			return true
		}
	}
	return false
}

// ── TEST-RES-01: minimal TSD → only core modules ─────────────────────────────

func TestResolve_MinimalTSD(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, validMinimal)
	for _, core := range []string{
		"core/multi-tenancy",
		"core/rbac",
		"core/audit-log",
		"core/feature-flags",
		"core/soft-delete",
	} {
		if !containsModule(modules, core) {
			t.Errorf("missing core module %q in %v", core, modules)
		}
	}
}

// ── TEST-RES-02: nextjs-15 frontend ──────────────────────────────────────────

func TestResolve_NextJS15Frontend(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  frontend:
    framework: nextjs-15
`)
	if !containsModule(modules, "frontend/nextjs-15-supabase") {
		t.Errorf("expected 'frontend/nextjs-15-supabase' in %v", modules)
	}
}

// ── TEST-RES-03: python+fastapi backend ──────────────────────────────────────

func TestResolve_PythonFastAPI(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  backend:
    language: python
    framework: fastapi
`)
	if !containsModule(modules, "backend/fastapi-supabase") {
		t.Errorf("expected 'backend/fastapi-supabase' in %v", modules)
	}
}

// ── TEST-RES-04: go+chi backend ──────────────────────────────────────────────

func TestResolve_GoChi(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  backend:
    language: go
    framework: chi
`)
	if !containsModule(modules, "backend/go-chi-supabase") {
		t.Errorf("expected 'backend/go-chi-supabase' in %v", modules)
	}
}

// ── TEST-RES-05: stripe payment provider ─────────────────────────────────────

func TestResolve_StripePayment(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  payments:
    providers: [stripe]
`)
	// RES-PAY-01: stripe maps to the frontend nextjs-supabase-stripe module (B1 regression fix).
	if !containsModule(modules, "frontend/nextjs-15-supabase-stripe") {
		t.Errorf("expected 'frontend/nextjs-15-supabase-stripe' in %v", modules)
	}
	// Guard: old (wrong) ID must NOT be present.
	if containsModule(modules, "backend/payments-stripe") {
		t.Errorf("unexpected stale module 'backend/payments-stripe' in %v", modules)
	}
}

// ── TEST-RES-06: docker-compose infra ────────────────────────────────────────

func TestResolve_DockerCompose(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  infra:
    container: docker-compose
`)
	if !containsModule(modules, "infra/docker-compose-fullstack") {
		t.Errorf("expected 'infra/docker-compose-fullstack' in %v", modules)
	}
}

// ── TEST-RES-07: langgraph AI ─────────────────────────────────────────────────

func TestResolve_LangGraph(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  ai:
    orchestration: langgraph
`)
	if !containsModule(modules, "backend/langgraph-agent") {
		t.Errorf("expected 'backend/langgraph-agent' in %v", modules)
	}
}

// ── TEST-RES-08: observability module always included ─────────────────────────

func TestResolve_ObservabilityAlwaysIncluded(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, validMinimal)
	if !containsModule(modules, "observability/structured-logging") {
		t.Errorf("expected 'observability/structured-logging' in %v", modules)
	}
}

// ── TEST-RES-09: observability is last ───────────────────────────────────────

func TestResolve_ObservabilityLast(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, validMinimal)
	if len(modules) == 0 {
		t.Fatal("empty module list")
	}
	if modules[len(modules)-1] != "observability/structured-logging" {
		t.Errorf("last module = %q, want 'observability/structured-logging'", modules[len(modules)-1])
	}
}

// ── TEST-RES-10: unknown frontend.framework → error ──────────────────────────

func TestResolve_UnknownFramework(t *testing.T) {
	t.Parallel()
	_, err := resolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  frontend:
    framework: angular
`)
	if err == nil {
		t.Fatal("expected error for unknown framework 'angular'")
	}
	var unknown *tsd.UnknownMappingError
	if !isUnknownMappingError(err, &unknown) {
		t.Errorf("want *tsd.UnknownMappingError, got %T: %v", err, err)
	}
}

// ── TEST-RES-11: idempotent — same TSD → same module list ────────────────────

func TestResolve_Idempotent(t *testing.T) {
	t.Parallel()
	a := mustResolve(t, validFull)
	b := mustResolve(t, validFull)
	if len(a) != len(b) {
		t.Fatalf("idempotency: len a=%d, len b=%d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("position %d: %q != %q", i, a[i], b[i])
		}
	}
}

// ── TEST-RES-12: core modules are always first 5 ─────────────────────────────

func TestResolve_CoreModulesFirst(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, validFull)
	coreModules := []string{
		"core/multi-tenancy",
		"core/rbac",
		"core/audit-log",
		"core/feature-flags",
		"core/soft-delete",
	}
	for i, m := range coreModules {
		if i >= len(modules) || modules[i] != m {
			t.Errorf("modules[%d] = %q, want %q", i, safeIndex(modules, i), m)
		}
	}
}

func safeIndex(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return "(out of bounds)"
}

// ── helpers ───────────────────────────────────────────────────────────────────

func isUnknownMappingError(err error, target **tsd.UnknownMappingError) bool {
	if err == nil {
		return false
	}
	ume, ok := err.(*tsd.UnknownMappingError)
	if ok && target != nil {
		*target = ume
	}
	return ok
}

// ── RES-PAY-01: paypal now maps to correct frontend module ────────────────────

func TestResolve_PayPalPayment(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  payments:
    providers: [paypal]
`)
	// RES-PAY-01: paypal maps to the frontend module.
	if !containsModule(modules, "frontend/nextjs-15-supabase-paypal") {
		t.Errorf("expected 'frontend/nextjs-15-supabase-paypal' in %v", modules)
	}
	// Guard: old (wrong) ID must NOT be present.
	if containsModule(modules, "backend/payments-paypal") {
		t.Errorf("unexpected stale module 'backend/payments-paypal' in %v", modules)
	}
}

// ── RES-PAY-02: both stripe and paypal resolve correctly ─────────────────────

func TestResolve_StripeAndPayPal(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  payments:
    providers: [stripe, paypal]
`)
	if !containsModule(modules, "frontend/nextjs-15-supabase-stripe") {
		t.Errorf("expected stripe module in %v", modules)
	}
	if !containsModule(modules, "frontend/nextjs-15-supabase-paypal") {
		t.Errorf("expected paypal module in %v", modules)
	}
}

// ── RES-PAY-03: empty providers → no payment modules ─────────────────────────

func TestResolve_NoPayments(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, validMinimal)
	for _, m := range modules {
		if m == "frontend/nextjs-15-supabase-stripe" || m == "frontend/nextjs-15-supabase-paypal" {
			t.Errorf("unexpected payment module %q in minimal TSD: %v", m, modules)
		}
	}
}

// ── RES-LLM-01: 2 LLM providers → fastapi-multi-llm included ─────────────────

func TestResolve_MultiLLM_TwoProviders(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  ai:
    llm_providers: [openai, anthropic]
`)
	if !containsModule(modules, "backend/fastapi-multi-llm") {
		t.Errorf("expected 'backend/fastapi-multi-llm' for 2 LLM providers in %v", modules)
	}
}

// ── RES-LLM-02: 3 LLM providers → fastapi-multi-llm included ─────────────────

func TestResolve_MultiLLM_ThreeProviders(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  ai:
    llm_providers: [openai, anthropic, google-gemini]
`)
	if !containsModule(modules, "backend/fastapi-multi-llm") {
		t.Errorf("expected 'backend/fastapi-multi-llm' for 3 LLM providers in %v", modules)
	}
}

// ── RES-LLM-03: 1 LLM provider → fastapi-multi-llm NOT included ──────────────

func TestResolve_SingleLLM_NoMultiModule(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  ai:
    llm_providers: [openai]
`)
	if containsModule(modules, "backend/fastapi-multi-llm") {
		t.Errorf("unexpected 'backend/fastapi-multi-llm' for single LLM provider in %v", modules)
	}
}

// ── RES-MQ-01: celery-redis queue → fastapi-redis-queue included ──────────────

func TestResolve_CeleryRedisQueue(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  messaging:
    queue: celery-redis
`)
	if !containsModule(modules, "backend/fastapi-redis-queue") {
		t.Errorf("expected 'backend/fastapi-redis-queue' for celery-redis queue in %v", modules)
	}
}

// ── RES-MQ-02: queue: none → no messaging module ─────────────────────────────

func TestResolve_NoQueue(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  messaging:
    queue: none
`)
	if containsModule(modules, "backend/fastapi-redis-queue") {
		t.Errorf("unexpected messaging module for queue: none in %v", modules)
	}
}

// ── RES-DO-01: cloud: digitalocean → digitalocean-app-platform included ──────

func TestResolve_DigitalOceanCloud(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  infra:
    cloud: digitalocean
`)
	if !containsModule(modules, "infra/digitalocean-app-platform") {
		t.Errorf("expected 'infra/digitalocean-app-platform' for digitalocean cloud in %v", modules)
	}
}

// ── RES-DO-02: cloud: aws → no module (forward-compat; no template yet) ──────

func TestResolve_AWSCloud_NoModule(t *testing.T) {
	t.Parallel()
	// aws maps to "" in infraCloudModuleMap — should not error.
	modules, err := resolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  infra:
    cloud: aws
`)
	if err != nil {
		t.Fatalf("unexpected error for cloud: aws: %v", err)
	}
	// No AWS-specific module expected yet.
	_ = modules
}

// ── RES-TG32-01: ci_cd: github-actions → infra/github-actions-ci ─────────────

func TestResolve_GithubActionsCICD(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  infra:
    ci_cd: github-actions
`)
	if !containsModule(modules, "infra/github-actions-ci") {
		t.Errorf("expected infra/github-actions-ci; got %v", modules)
	}
}

// ── RES-TG32-02: ci_cd: none → no github-actions-ci module ──────────────────

func TestResolve_NoCICD(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  infra:
    ci_cd: none
`)
	if containsModule(modules, "infra/github-actions-ci") {
		t.Errorf("unexpected infra/github-actions-ci for ci_cd: none; got %v", modules)
	}
}

// ── RES-TG32-03: tracing: opentelemetry → observability/otel-prometheus ──────

func TestResolve_OpenTelemetryTracing(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  observability:
    tracing: opentelemetry
`)
	if !containsModule(modules, "observability/otel-prometheus") {
		t.Errorf("expected observability/otel-prometheus; got %v", modules)
	}
}

// ── RES-TG32-04: metrics: prometheus-grafana → observability/otel-prometheus ─

func TestResolve_PrometheusMetrics(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  observability:
    metrics: prometheus-grafana
`)
	if !containsModule(modules, "observability/otel-prometheus") {
		t.Errorf("expected observability/otel-prometheus; got %v", modules)
	}
}

// ── RES-TG32-05: tracing + metrics both otel → no duplicates ─────────────────

func TestResolve_TracingAndMetricsNoDuplicates(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "test"
stack:
  observability:
    tracing: opentelemetry
    metrics: prometheus-grafana
`)
	count := 0
	for _, m := range modules {
		if m == "observability/otel-prometheus" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("observability/otel-prometheus included %d times (want ≤1); modules=%v", count, modules)
	}
}

// ── RES-TG32-06: PromotAI full stack resolves all expected modules ─────────────

func TestResolve_PromotAI_FullStack(t *testing.T) {
	t.Parallel()
	modules := mustResolve(t, `tsd_version: 1
project:
  name: "promotiai"
  type: saas
stack:
  frontend:
    framework: nextjs-15
  backend:
    language: python
    framework: fastapi
  ai:
    orchestration: langgraph
    llm_providers: [openai, anthropic, google-gemini]
  payments:
    providers: [stripe, paypal]
  messaging:
    queue: celery-redis
  infra:
    cloud: digitalocean
    container: docker-compose
    ci_cd: github-actions
  observability:
    tracing: opentelemetry
    metrics: prometheus-grafana
    logging: structlog
`)
	want := []string{
		"core/multi-tenancy",
		"core/rbac",
		"frontend/nextjs-15-supabase",
		"backend/fastapi-supabase",
		"backend/langgraph-agent",
		"backend/fastapi-multi-llm",
		"frontend/nextjs-15-supabase-stripe",
		"frontend/nextjs-15-supabase-paypal",
		"infra/docker-compose-fullstack",
		"infra/digitalocean-app-platform",
		"backend/fastapi-redis-queue",
		"infra/github-actions-ci",
		"observability/otel-prometheus",
		"observability/structured-logging",
	}
	for _, w := range want {
		if !containsModule(modules, w) {
			t.Errorf("expected module %q in result; got %v", w, modules)
		}
	}
}
