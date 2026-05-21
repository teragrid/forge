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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/tsd"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func parse(t *testing.T, yaml string) (*tsd.TSD, error) {
	t.Helper()
	return tsd.Parse(strings.NewReader(yaml))
}

func mustParse(t *testing.T, yaml string) *tsd.TSD {
	t.Helper()
	got, err := parse(t, yaml)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	return got
}

const validFull = `
tsd_version: 1
project:
  name: "acme-saas"
  domain: "acme.com"
  type: saas
  description: "Test project"
stack:
  frontend:
    framework: nextjs-15
    ui_library: shadcn
    state_management: server-components-only
    testing: jest-rtl
  backend:
    language: python
    framework: fastapi
    api_style: rest
    auth: supabase-auth
    testing: pytest
  database:
    primary: supabase-pg
    cache: redis
    search: pg-fts
    migrations: supabase-migrations
  ai:
    orchestration: langgraph
    llm_providers: [openai, anthropic]
    embedding: openai-ada
    vector_store: supabase-pgvector
    observability: langsmith
  payments:
    providers: [stripe, paypal]
    model: subscription
  infra:
    cloud: digitalocean
    container: docker-compose
    cdn: cloudflare
    secrets: doppler
    ci_cd: github-actions
  compliance:
    standards: [gdpr, pci-dss-saq-a]
    secret_scanning: gitleaks
`

const validMinimal = `
tsd_version: 1
project:
  name: "minimal-project"
`

// ── TEST-TSD-01: parse valid full TSD ────────────────────────────────────────

func TestParse_ValidFull(t *testing.T) {
	t.Parallel()
	got := mustParse(t, validFull)
	if got.Project.Name != "acme-saas" {
		t.Errorf("project.name = %q, want acme-saas", got.Project.Name)
	}
	if got.Stack.Frontend.Framework != "nextjs-15" {
		t.Errorf("frontend.framework = %q, want nextjs-15", got.Stack.Frontend.Framework)
	}
	if len(got.Stack.AI.LLMProviders) != 2 {
		t.Errorf("llm_providers len = %d, want 2", len(got.Stack.AI.LLMProviders))
	}
	if len(got.UnknownKeys) != 0 {
		t.Errorf("unexpected unknown keys: %v", got.UnknownKeys)
	}
}

// ── TEST-TSD-02: parse minimal TSD ───────────────────────────────────────────

func TestParse_Minimal(t *testing.T) {
	t.Parallel()
	got := mustParse(t, validMinimal)
	if got.Project.Name != "minimal-project" {
		t.Errorf("project.name = %q", got.Project.Name)
	}
	// All stack fields should be zero-value strings.
	if got.Stack.Frontend.Framework != "" {
		t.Errorf("expected empty frontend.framework, got %q", got.Stack.Frontend.Framework)
	}
	if len(got.UnknownKeys) != 0 {
		t.Errorf("unexpected unknown keys: %v", got.UnknownKeys)
	}
}

// ── TEST-TSD-03: AI section omitted → zero-value, no error ───────────────────

func TestParse_AIAbsent(t *testing.T) {
	t.Parallel()
	yaml := `
tsd_version: 1
project:
  name: "no-ai-project"
stack:
  frontend:
    framework: nextjs-15
`
	got := mustParse(t, yaml)
	if got.Stack.AI.Orchestration != "" {
		t.Errorf("expected empty ai.orchestration, got %q", got.Stack.AI.Orchestration)
	}
	if len(got.Stack.AI.LLMProviders) != 0 {
		t.Errorf("expected empty llm_providers, got %v", got.Stack.AI.LLMProviders)
	}
}

// ── TEST-TSD-04: unknown top-level key collected as warning ───────────────────

func TestParse_UnknownTopLevelKey(t *testing.T) {
	t.Parallel()
	yaml := `
tsd_version: 1
project:
  name: "test"
custom_section:
  foo: bar
`
	got := mustParse(t, yaml)
	found := false
	for _, k := range got.UnknownKeys {
		if k == "custom_section" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'custom_section' in UnknownKeys, got %v", got.UnknownKeys)
	}
}

// ── TEST-TSD-04b: unknown stack-level key collected as warning ────────────────

func TestParse_UnknownStackKey(t *testing.T) {
	t.Parallel()
	yaml := `
tsd_version: 1
project:
  name: "test"
stack:
  custom_field: x
`
	got := mustParse(t, yaml)
	found := false
	for _, k := range got.UnknownKeys {
		if k == "stack.custom_field" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'stack.custom_field' in UnknownKeys, got %v", got.UnknownKeys)
	}
}

// ── TEST-TSD-05: wrong tsd_version → ErrUnsupportedVersion ───────────────────

func TestParse_WrongVersion(t *testing.T) {
	t.Parallel()
	_, err := parse(t, "tsd_version: 99\nproject:\n  name: x\n")
	if err == nil {
		t.Fatal("expected error for version 99, got nil")
	}
	// ErrUnsupportedVersion is an errcode.Code; check by message substring.
	if !strings.Contains(err.Error(), "unsupported TSD schema version") {
		t.Errorf("want ErrUnsupportedVersion message, got %v", err)
	}
}

// ── TEST-TSD-06: empty file → error ──────────────────────────────────────────

func TestParse_EmptyFile(t *testing.T) {
	t.Parallel()
	_, err := parse(t, "")
	if err == nil {
		t.Fatal("expected error for empty file, got nil")
	}
}

// ── TEST-TSD-07: non-YAML content → parse error ──────────────────────────────

func TestParse_NonYAML(t *testing.T) {
	t.Parallel()
	_, err := parse(t, "not: valid: yaml: :")
	if err == nil {
		t.Fatal("expected YAML parse error, got nil")
	}
}

// ── TEST-TSD-08: providers as list ───────────────────────────────────────────

func TestParse_ProvidersAsList(t *testing.T) {
	t.Parallel()
	yaml := `
tsd_version: 1
project:
  name: "pay-test"
stack:
  payments:
    providers: [stripe, paypal]
`
	got := mustParse(t, yaml)
	if len(got.Stack.Payments.Providers) != 2 {
		t.Errorf("providers len = %d, want 2", len(got.Stack.Payments.Providers))
	}
}

// ── TEST-TSD-09: empty providers list → no error ─────────────────────────────

func TestParse_EmptyProviders(t *testing.T) {
	t.Parallel()
	yaml := `
tsd_version: 1
project:
  name: "no-pay"
stack:
  payments:
    providers: []
`
	got := mustParse(t, yaml)
	if len(got.Stack.Payments.Providers) != 0 {
		t.Errorf("expected empty providers, got %v", got.Stack.Payments.Providers)
	}
}

// ── TEST-TSD-10: ParseFile helper ────────────────────────────────────────────

func TestParseFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "tsd.yml")
	if err := os.WriteFile(path, []byte(validFull), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := tsd.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if got.Project.Name != "acme-saas" {
		t.Errorf("project.name = %q, want acme-saas", got.Project.Name)
	}
}

// ── Idempotency: parse same content twice → identical ────────────────────────

func TestParse_Idempotency(t *testing.T) {
	t.Parallel()
	a := mustParse(t, validFull)
	b := mustParse(t, validFull)
	if a.Project.Name != b.Project.Name ||
		a.Stack.Frontend.Framework != b.Stack.Frontend.Framework ||
		len(a.Stack.Payments.Providers) != len(b.Stack.Payments.Providers) {
		t.Error("parse is not idempotent")
	}
}

// ── Regression: unknown keys must NOT cause an error ─────────────────────────

func TestParse_UnknownKeyNoError(t *testing.T) {
	// regression: unknown keys must never cause error (forward-compat requirement)
	t.Parallel()
	yaml := `
tsd_version: 1
project:
  name: "forward-compat"
future_section:
  cool_new_feature: true
stack:
  new_stack_key: something
`
	got, err := parse(t, yaml)
	if err != nil {
		t.Fatalf("unknown keys must not cause error, got: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil TSD")
	}
	if len(got.UnknownKeys) == 0 {
		t.Error("expected unknown keys to be collected")
	}
}
