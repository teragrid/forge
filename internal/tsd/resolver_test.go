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
	if !containsModule(modules, "backend/payments-stripe") {
		t.Errorf("expected 'backend/payments-stripe' in %v", modules)
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
