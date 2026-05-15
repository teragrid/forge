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

package cmdship

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExtractSpecFields verifies that acceptance criteria, events, and authz
// are correctly extracted from Markdown spec content.
func TestExtractSpecFields(t *testing.T) {
	md := `# Feature: Login

## Acceptance Criteria

- [ ] Given a valid user, when they log in, then they receive a token
- [ ] Given an invalid password, when they log in, then they see an error
- [x] Given a locked account, when they log in, then access is denied

event: UserLoggedIn
event: LoginFailed
authz: RBAC with role=user required

## Out of scope
- SSO
`
	criteria, events, authz := extractSpecFields(md)
	if len(criteria) != 3 {
		t.Errorf("criteria: got %d, want 3: %v", len(criteria), criteria)
	}
	if len(events) != 2 {
		t.Errorf("events: got %d, want 2: %v", len(events), events)
	}
	if authz != "RBAC with role=user required" {
		t.Errorf("authz: got %q", authz)
	}
}

// TestExtractSpecFields_Empty verifies that empty Markdown returns empty slices.
func TestExtractSpecFields_Empty(t *testing.T) {
	criteria, events, authz := extractSpecFields("")
	if len(criteria) != 0 || len(events) != 0 || authz != "" {
		t.Errorf("expected all empty; got criteria=%v events=%v authz=%q", criteria, events, authz)
	}
}

// TestNewSpecManifest checks that newSpecManifest populates required fields.
func TestNewSpecManifest(t *testing.T) {
	m := newSpecManifest("auth-login", "auth login", "- [ ] Given a user")
	if m.SchemaVersion != 1 {
		t.Errorf("schema_version: got %d, want 1", m.SchemaVersion)
	}
	if m.ID != "auth-login" {
		t.Errorf("id: got %q", m.ID)
	}
	if m.Feature != "auth login" {
		t.Errorf("feature: got %q", m.Feature)
	}
	if m.Status != "draft" {
		t.Errorf("status: got %q", m.Status)
	}
	if m.CreatedAt == "" {
		t.Error("created_at should be set")
	}
	if len(m.ScanPolicy.Families) != len(defaultScanFamilies) {
		t.Errorf("scan families: got %d, want %d", len(m.ScanPolicy.Families), len(defaultScanFamilies))
	}
	if len(m.AcceptanceCriteria) != 1 {
		t.Errorf("acceptance_criteria: got %d, want 1", len(m.AcceptanceCriteria))
	}
}

// TestMarshalSpecManifest checks that marshalSpecManifest produces valid YAML
// with the expected header comment and key fields.
func TestMarshalSpecManifest(t *testing.T) {
	m := newSpecManifest("my-feature", "my feature", "")
	data, err := marshalSpecManifest(m)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "# forge spec manifest") {
		t.Error("missing header comment")
	}
	if !strings.Contains(s, "schema_version: 1") {
		t.Error("missing schema_version field")
	}
	if !strings.Contains(s, "id: my-feature") {
		t.Error("missing id field")
	}
	if !strings.Contains(s, "feature: my feature") {
		t.Error("missing feature field")
	}
}

// TestWriteSpecManifest verifies that writeSpecManifest creates spec.yml in
// the expected location with correct content.
func TestWriteSpecManifest(t *testing.T) {
	dir := t.TempDir()
	slug := "write-test"
	specsDir := dir
	md := "- [ ] Given something, when something, then something\nevent: TestEvent"

	writeSpecManifest(specsDir, slug, "write test", md)

	ymlPath := filepath.Join(specsDir, slug, "spec.yml")
	data, err := os.ReadFile(ymlPath)
	if err != nil {
		t.Fatalf("spec.yml not created: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "id: write-test") {
		t.Errorf("spec.yml missing id: %s", s)
	}
	if !strings.Contains(s, "feature: write test") {
		t.Errorf("spec.yml missing feature: %s", s)
	}
	if !strings.Contains(s, "TestEvent") {
		t.Errorf("spec.yml missing event: %s", s)
	}
}

// TestLoadSpecManifest verifies round-trip write + read.
func TestLoadSpecManifest(t *testing.T) {
	dir := t.TempDir()
	slug := "load-test"

	writeSpecManifest(dir, slug, "load test feature", "- [ ] some criterion")

	m := loadSpecManifest(dir, slug)
	if m == nil {
		t.Fatal("loadSpecManifest returned nil")
	}
	if m.ID != slug {
		t.Errorf("id: got %q, want %q", m.ID, slug)
	}
	if m.Feature != "load test feature" {
		t.Errorf("feature: got %q", m.Feature)
	}
	if m.SchemaVersion != 1 {
		t.Errorf("schema_version: got %d", m.SchemaVersion)
	}
}

// TestLoadSpecManifest_Missing returns nil for non-existent slug.
func TestLoadSpecManifest_Missing(t *testing.T) {
	dir := t.TempDir()
	if m := loadSpecManifest(dir, "no-such-slug"); m != nil {
		t.Errorf("expected nil for missing slug, got %+v", m)
	}
}
