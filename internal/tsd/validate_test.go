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

func validate(t *testing.T, yaml string) []tsd.ValidationError {
	t.Helper()
	parsed, err := tsd.Parse(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	return tsd.Validate(parsed)
}

func assertNoErrors(t *testing.T, errs []tsd.ValidationError) {
	t.Helper()
	if len(errs) != 0 {
		t.Errorf("expected no validation errors, got %d: %v", len(errs), errs)
	}
}

func assertHasFieldError(t *testing.T, errs []tsd.ValidationError, field string) {
	t.Helper()
	for _, e := range errs {
		if e.Field == field || strings.HasPrefix(e.Field, field) {
			return
		}
	}
	t.Errorf("expected error for field %q, got errors: %v", field, errs)
}

// ── TEST-VAL-01: fully valid TSD ─────────────────────────────────────────────

func TestValidate_FullyValid(t *testing.T) {
	t.Parallel()
	errs := validate(t, validFull) // uses validFull defined in tsd_test.go
	assertNoErrors(t, errs)
}

// ── TEST-VAL-02: missing project.name ────────────────────────────────────────

func TestValidate_MissingProjectName(t *testing.T) {
	t.Parallel()
	errs := validate(t, `tsd_version: 1
project:
  name: ""
  type: saas
`)
	assertHasFieldError(t, errs, "project.name")
	if len(errs) == 0 {
		t.Fatal("expected at least one error")
	}
	if errs[0].Code != 6400 {
		t.Errorf("want code 6400, got %d", errs[0].Code)
	}
}

// ── TEST-VAL-03: invalid project.type ────────────────────────────────────────

func TestValidate_InvalidProjectType(t *testing.T) {
	t.Parallel()
	errs := validate(t, `tsd_version: 1
project:
  name: "test"
  type: blog
`)
	assertHasFieldError(t, errs, "project.type")
	// Error message should list allowed values.
	if !strings.Contains(errs[0].Message, "allowed:") {
		t.Errorf("error message should list allowed values: %s", errs[0].Message)
	}
}

// ── TEST-VAL-04: invalid frontend.framework ───────────────────────────────────

func TestValidate_InvalidFrontendFramework(t *testing.T) {
	t.Parallel()
	errs := validate(t, `tsd_version: 1
project:
  name: "test"
stack:
  frontend:
    framework: angular
`)
	assertHasFieldError(t, errs, "stack.frontend.framework")
}

// ── TEST-VAL-05: invalid backend.language ────────────────────────────────────

func TestValidate_InvalidBackendLanguage(t *testing.T) {
	t.Parallel()
	errs := validate(t, `tsd_version: 1
project:
  name: "test"
stack:
  backend:
    language: ruby
`)
	assertHasFieldError(t, errs, "stack.backend.language")
}

// ── TEST-VAL-06: multiple invalid fields → one error per field ───────────────

func TestValidate_MultipleInvalidFields(t *testing.T) {
	t.Parallel()
	errs := validate(t, `tsd_version: 1
project:
  name: "test"
  type: blog
stack:
  backend:
    language: ruby
`)
	if len(errs) < 2 {
		t.Errorf("expected >= 2 errors, got %d: %v", len(errs), errs)
	}
}

// ── TEST-VAL-07: empty payments.providers → no error ─────────────────────────

func TestValidate_EmptyPaymentProviders(t *testing.T) {
	t.Parallel()
	errs := validate(t, `tsd_version: 1
project:
  name: "test"
stack:
  payments:
    providers: []
`)
	assertNoErrors(t, errs)
}

// ── TEST-VAL-08: ai section absent → no error ────────────────────────────────

func TestValidate_AIAbsent(t *testing.T) {
	t.Parallel()
	errs := validate(t, validMinimal)
	assertNoErrors(t, errs)
}

// ── TEST-VAL-09: ai.orchestration = none → no error ──────────────────────────

func TestValidate_AINone(t *testing.T) {
	t.Parallel()
	errs := validate(t, `tsd_version: 1
project:
  name: "test"
stack:
  ai:
    orchestration: none
`)
	assertNoErrors(t, errs)
}

// ── TEST-VAL-10: unknown stack key → warning not error ───────────────────────

func TestValidate_UnknownStackKeyIsWarning(t *testing.T) {
	t.Parallel()
	parsed, err := tsd.Parse(strings.NewReader(`tsd_version: 1
project:
  name: "test"
stack:
  custom_key: foo
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	errs := tsd.Validate(parsed)
	assertNoErrors(t, errs)
	// Unknown key should be in UnknownKeys, not in validation errors.
	found := false
	for _, k := range parsed.UnknownKeys {
		if strings.Contains(k, "custom_key") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'custom_key' in UnknownKeys, got %v", parsed.UnknownKeys)
	}
}

// ── TEST-VAL-12: invalid compliance standard ──────────────────────────────────

func TestValidate_InvalidComplianceStandard(t *testing.T) {
	t.Parallel()
	errs := validate(t, `tsd_version: 1
project:
  name: "test"
stack:
  compliance:
    standards: [hipaa-v2]
`)
	assertHasFieldError(t, errs, "stack.compliance.standards[]")
}

// ── TEST-VAL-13: false-positive guard — valid edge values → zero errors ───────

func TestValidate_FalsePositiveGuard(t *testing.T) {
	t.Parallel()
	// Use the last enum option of each field to verify full enum coverage.
	errs := validate(t, `tsd_version: 1
project:
  name: "edge-case"
  type: marketplace
stack:
  frontend:
    framework: vue-3
    ui_library: none
  backend:
    language: java
    framework: none
    api_style: trpc
    auth: none
  database:
    primary: none
    cache: none
  ai:
    orchestration: none
  payments:
    providers: []
    model: marketplace
  infra:
    cloud: none
    container: none
    ci_cd: none
  compliance:
    standards: [soc2]
`)
	assertNoErrors(t, errs)
}

// ── Validate nil TSD → safe ───────────────────────────────────────────────────

func TestValidate_Nil(t *testing.T) {
	t.Parallel()
	errs := tsd.Validate(nil)
	if len(errs) == 0 {
		t.Error("expected error for nil TSD")
	}
}
