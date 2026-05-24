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

package cmdship

// Test design for arch checkpoint (checkArch):
//
// 1. Happy path              — arch.md written from LLM-generated content; openapi.yaml also written.
// 2. Idempotent              — existing arch.md returns "ok" immediately without LLM call;
//                              openapi.yaml stub back-filled if absent.
// 3. No spec                 — returns "warning" with hint to run spec first.
// 4. No description          — returns "warning" asking for --description.
// 5. No description with LLM — returns "warning" with provider name in detail.
// 6. LLM error / graceful    — provider failure → stub arch.md + openapi.yaml still written, status ok.
// 7. No LLM pipe (stub mode) — stub arch.md + openapi.yaml written; status ok; detail mentions ANTHROPIC_API_KEY.
// 8. openapiStub             — stub has all required OpenAPI 3.1.0 fields.
// 9. extractOpenAPIBlock     — extracts YAML block from LLM response; remainder becomes arch.md.
// 10. Arch in full pipeline  — 6-checkpoint pipeline includes arch at index 1.
// 11. SelfDebate arch roles  — archCatalog populated; 3-round debate produces improvements.
// 12. checkpointIndex        — RunCheckpoints(names=["arch"]) resolves to checkArch result.
// 13. False-positive guard   — arch warning does not block the pipeline.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/llmprovider"
)

// ── Happy path ────────────────────────────────────────────────────────────────

// TestCheckArch_LLM_GeneratesArchDoc — when spec.md exists and no arch.md is
// present, the LLM generates an arch document which is written to
// .forge/specs/<slug>/arch.md.
func TestCheckArch_LLM_GeneratesArchDoc(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := slugify("add payment api")
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec: add payment API\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	mock := &llmprovider.MockProvider{
		Response: mockResponse("# Architecture: add payment API\n\n## 1. Component Topology\n\nPayment service.\n"),
	}
	cp := checkArch(root, "add payment api", mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "arch") {
		t.Errorf("detail should reference arch file: %s", cp.Detail)
	}
	data, err := os.ReadFile(filepath.Join(dir, "arch.md"))
	if err != nil {
		t.Fatalf("arch.md not written: %v", err)
	}
	if !strings.Contains(string(data), "payment") {
		t.Errorf("arch.md should contain feature text: %s", string(data))
	}
	// openapi.yaml must also be written alongside arch.md.
	openapiData, openapiErr := os.ReadFile(filepath.Join(dir, "openapi.yaml"))
	if openapiErr != nil {
		t.Fatalf("openapi.yaml not written by checkArch: %v", openapiErr)
	}
	if !strings.Contains(string(openapiData), "openapi") {
		t.Errorf("openapi.yaml should contain 'openapi' field, got: %s", string(openapiData))
	}
	if mock.Calls() == 0 {
		t.Error("MockProvider.Complete was not called")
	}
}

// ── Idempotency ───────────────────────────────────────────────────────────────

// TestCheckArch_ExistingArch_Idempotent — when arch.md already exists the
// checkpoint returns "ok" immediately without calling the LLM.
func TestCheckArch_ExistingArch_Idempotent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := slugify("existing arch feature")
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "arch.md"), []byte("# Arch already present\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	mock := &llmprovider.MockProvider{Response: mockResponse("should not be called")}
	cp := checkArch(root, "existing arch feature", mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("expected ok for existing arch, got %q: %s", cp.Status, cp.Detail)
	}
	if mock.Calls() > 0 {
		t.Error("MockProvider should not be called when arch.md already exists")
	}
	if !strings.Contains(cp.Detail, "arch document found") {
		t.Errorf("detail should say 'arch document found': %s", cp.Detail)
	}
	// openapi.yaml stub must be back-filled when it was absent.
	openapiPath := filepath.Join(dir, "openapi.yaml")
	if _, err := os.Stat(openapiPath); err != nil {
		t.Error("openapi.yaml should be back-filled when arch.md exists but openapi.yaml is absent")
	}
}

// ── No spec ───────────────────────────────────────────────────────────────────

// TestCheckArch_NoSpec_Warning — when spec.md does not exist the checkpoint
// returns "warning" with a hint to run forge ship spec first.
func TestCheckArch_NoSpec_Warning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{Response: mockResponse("should not reach LLM")}
	cp := checkArch(root, "missing spec feature", mockPipe(root, mock))

	if cp.Status != "warning" {
		t.Fatalf("expected warning when no spec, got %q: %s", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "spec") {
		t.Errorf("detail should mention spec: %s", cp.Detail)
	}
}

// ── No description ────────────────────────────────────────────────────────────

// TestCheckArch_NoDescription_NoLLM_Warning — empty description without LLM
// returns "warning" with a hint to pass --description.
func TestCheckArch_NoDescription_NoLLM_Warning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cp := checkArch(root, "", nil)

	if cp.Status != "warning" {
		t.Fatalf("expected warning with no description, got %q: %s", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "--description") && !strings.Contains(cp.Detail, "description") {
		t.Errorf("detail should mention description: %s", cp.Detail)
	}
}

// TestCheckArch_NoDescription_WithLLM_Warning — empty description with LLM
// still returns "warning" and includes the provider name in the detail.
func TestCheckArch_NoDescription_WithLLM_Warning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mock := &llmprovider.MockProvider{Response: mockResponse("ignored")}
	cp := checkArch(root, "", mockPipe(root, mock))

	if cp.Status != "warning" {
		t.Fatalf("expected warning with no description + LLM, got %q: %s", cp.Status, cp.Detail)
	}
	if !strings.Contains(cp.Detail, "mock") {
		t.Errorf("detail should mention provider name: %s", cp.Detail)
	}
}

// ── LLM error → graceful degradation ─────────────────────────────────────────

// TestCheckArch_LLMError_StubWritten — when the LLM returns an error, a stub
// arch.md is still written and the checkpoint reports "ok" (graceful fallback).
func TestCheckArch_LLMError_StubWritten(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := slugify("payment feature")
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	mock := &llmprovider.MockProvider{Err: fmt.Errorf("FORGE-4051 transport not implemented")}
	cp := checkArch(root, "payment feature", mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("LLM error should not fail the arch checkpoint; got %q: %s", cp.Status, cp.Detail)
	}
	archPath := filepath.Join(dir, "arch.md")
	if _, err := os.Stat(archPath); err != nil {
		t.Fatal("arch.md stub should be written even when LLM fails")
	}
	// openapi.yaml stub must also be written on LLM error.
	openapiPath := filepath.Join(dir, "openapi.yaml")
	if _, err := os.Stat(openapiPath); err != nil {
		t.Fatal("openapi.yaml stub should be written even when LLM fails")
	}
	if !strings.Contains(cp.Detail, "stub") {
		t.Errorf("detail should mention stub on LLM error: %s", cp.Detail)
	}
}

// ── No LLM pipe (stub-only mode) ─────────────────────────────────────────────

// TestCheckArch_NoLLM_StubWritten — without an LLMPipe, a structured stub is
// written and the checkpoint returns "ok" with a hint about ANTHROPIC_API_KEY.
func TestCheckArch_NoLLM_StubWritten(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := slugify("new feature")
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cp := checkArch(root, "new feature", nil)

	if cp.Status != "ok" {
		t.Fatalf("expected ok without LLM, got %q: %s", cp.Status, cp.Detail)
	}
	archPath := filepath.Join(dir, "arch.md")
	data, err := os.ReadFile(archPath)
	if err != nil {
		t.Fatalf("arch.md not written without LLM: %v", err)
	}
	if !strings.Contains(string(data), "Architecture") {
		t.Errorf("stub arch.md should contain 'Architecture': %s", string(data))
	}
	// openapi.yaml must also be written without LLM.
	openapiPath := filepath.Join(dir, "openapi.yaml")
	openapiData, openapiErr := os.ReadFile(openapiPath)
	if openapiErr != nil {
		t.Fatalf("openapi.yaml not written without LLM: %v", openapiErr)
	}
	if !strings.Contains(string(openapiData), "openapi:") {
		t.Errorf("openapi.yaml stub should contain 'openapi:' field: %s", string(openapiData))
	}
	if !strings.Contains(cp.Detail, "forge config set llm.provider") {
		t.Errorf("detail should hint about forge config set llm.provider: %s", cp.Detail)
	}
}

// ── Arch stub content ─────────────────────────────────────────────────────────

// TestArchStub_ContainsRequiredSections — archStub must include all 6 required
// ADR sections and the ADR summary block.
func TestArchStub_ContainsRequiredSections(t *testing.T) {
	t.Parallel()
	stub := archStub("test feature")

	sections := []string{
		"Component Topology",
		"API Contracts",
		"openapi.yaml",
		"Data Model",
		"Non-Functional Requirements",
		"Security Threat Model",
		"Deployment",
		"ADR Summary",
	}
	for _, s := range sections {
		if !strings.Contains(stub, s) {
			t.Errorf("arch stub missing section/keyword %q", s)
		}
	}
}

// TestOpenapiStub_ContainsRequiredFields — openapiStub must produce a valid
// OpenAPI 3.1.0 skeleton with all required top-level fields.
func TestOpenapiStub_ContainsRequiredFields(t *testing.T) {
	t.Parallel()
	stub := openapiStub("test feature")

	required := []string{
		`openapi: "3.1.0"`,
		"info:",
		"title:",
		"version:",
		"paths:",
		"components:",
		"schemas:",
		"TODORequest",
		"TODOResponse",
		"test feature", // feature name embedded in info
	}
	for _, field := range required {
		if !strings.Contains(stub, field) {
			t.Errorf("openapiStub missing required field/keyword %q", field)
		}
	}
}

// TestExtractOpenAPIBlock_ExtractsYAMLBlock — given an LLM response that
// embeds a ```yaml openapi block, extractOpenAPIBlock splits it correctly.
func TestExtractOpenAPIBlock_ExtractsYAMLBlock(t *testing.T) {
	t.Parallel()
	input := `# Architecture: my feature

## 1. Component Topology

Service A -> Service B.

` + "```yaml\nopenapi: \"3.1.0\"\ninfo:\n  title: My API\npaths: {}\ncomponents:\n  schemas: {}\n```" + `
`
	archMD, openapiYAML := extractOpenAPIBlock(input, "my feature")

	if !strings.Contains(archMD, "Component Topology") {
		t.Errorf("archMD should contain the ADR content, got: %s", archMD)
	}
	if strings.Contains(archMD, "```yaml") {
		t.Error("archMD should not contain the raw ```yaml block")
	}
	if !strings.Contains(archMD, "openapi.yaml") {
		t.Errorf("archMD should reference openapi.yaml file: %s", archMD)
	}
	if !strings.Contains(openapiYAML, `openapi: "3.1.0"`) {
		t.Errorf("openapiYAML should contain openapi version, got: %s", openapiYAML)
	}
}

// TestExtractOpenAPIBlock_NoYAMLBlock_ReturnsStub — when no ```yaml block
// with openapi: is present, the full content stays as arch.md and a stub
// openapi.yaml is returned (no data loss).
func TestExtractOpenAPIBlock_NoYAMLBlock_ReturnsStub(t *testing.T) {
	t.Parallel()
	input := "# Architecture\n\nNo API block here.\n"
	archMD, openapiYAML := extractOpenAPIBlock(input, "no api")

	if archMD != input {
		t.Errorf("archMD should be unchanged when no YAML block found")
	}
	if !strings.Contains(openapiYAML, "openapi:") {
		t.Errorf("should return openapiStub when no block found: %s", openapiYAML)
	}
}

// ── Arch in full pipeline ─────────────────────────────────────────────────────

// TestRunCheckpoints_ArchInPipeline — arch checkpoint at index 1 (after spec)
// is included in a full 6-checkpoint pipeline run.
func TestRunCheckpoints_ArchInPipeline(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	res := RunCheckpoints(root, "arch pipeline test", nil)

	if len(res.Checkpoints) < 6 {
		t.Fatalf("expected 6 checkpoints, got %d", len(res.Checkpoints))
	}
	// Checkpoint index 1 must be the Arch checkpoint.
	if res.Checkpoints[1].Name != "Arch" {
		t.Errorf("checkpoint[1] should be Arch, got %q", res.Checkpoints[1].Name)
	}
}

// ── checkpointIndex resolution ────────────────────────────────────────────────

// TestRunCheckpoints_ArchByName — RunCheckpoints with names=["arch"] runs
// only the arch checkpoint and returns a single result.
func TestRunCheckpoints_ArchByName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := slugify("arch by name")
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	res := RunCheckpoints(root, "arch by name", []string{"arch"})

	if len(res.Checkpoints) != 1 {
		t.Fatalf("expected 1 checkpoint result, got %d", len(res.Checkpoints))
	}
	if res.Checkpoints[0].Name != "Arch" {
		t.Errorf("single checkpoint should be Arch, got %q", res.Checkpoints[0].Name)
	}
}

// ── Self-debate arch roles ─────────────────────────────────────────────────────

// TestSelfDebate_ArchDeliverable_SixRoles — SelfDebate with deliverable="arch"
// uses DefaultArchRoles and raises concerns from archCatalog.
func TestSelfDebate_ArchDeliverable_SixRoles(t *testing.T) {
	t.Parallel()
	opts := DebateOptions{
		Deliverable: "arch",
		Feature:     "payment gateway",
		Roles:       DefaultArchRoles(),
		MaxRounds:   3,
		DryRun:      true,
	}
	result := SelfDebate(opts)

	if result == nil {
		t.Fatal("SelfDebate returned nil")
	}
	if len(result.Roles) != 6 {
		t.Errorf("expected 6 arch roles, got %d", len(result.Roles))
	}
	if !result.Consensus {
		t.Error("dry-run debate should always reach consensus")
	}
	if len(result.Rounds) != 3 {
		t.Errorf("expected 3 debate rounds, got %d", len(result.Rounds))
	}
	if len(result.Improvements) == 0 {
		t.Error("expected at least 1 improvement from arch debate")
	}
	// Polished summary must mention arch.
	if !strings.Contains(result.PolishedSummary, "arch") {
		t.Errorf("polished summary should mention deliverable 'arch': %s", result.PolishedSummary)
	}
}

// TestDefaultArchRoles_SixDistinctRoles — DefaultArchRoles returns exactly 6
// roles with distinct IDs, all different from the 8 general roles.
func TestDefaultArchRoles_SixDistinctRoles(t *testing.T) {
	t.Parallel()
	archRoles := DefaultArchRoles()
	if len(archRoles) != 6 {
		t.Fatalf("expected 6 arch roles, got %d", len(archRoles))
	}

	generalIDs := map[RoleID]bool{
		RolePO: true, RoleBA: true, RoleSA: true, RoleDL: true,
		RoleQE: true, RoleSec: true, RoleOps: true, RoleCPO: true,
	}
	seen := map[RoleID]bool{}
	for _, r := range archRoles {
		if seen[r.ID] {
			t.Errorf("duplicate arch role ID: %s", r.ID)
		}
		seen[r.ID] = true
		if generalIDs[r.ID] {
			t.Errorf("arch role %s overlaps with a general role ID", r.ID)
		}
		if r.Name == "" {
			t.Errorf("arch role %s has empty Name", r.ID)
		}
		if r.Hat == "" {
			t.Errorf("arch role %s has empty Hat", r.ID)
		}
		if len(r.FocusAreas) == 0 {
			t.Errorf("arch role %s has no FocusAreas", r.ID)
		}
	}
}

// TestArchCatalog_AllRolesHaveConcerns — archCatalog must have entries for
// every arch role and each role must have at least 2 concerns.
func TestArchCatalog_AllRolesHaveConcerns(t *testing.T) {
	t.Parallel()
	for _, r := range DefaultArchRoles() {
		concerns := archCatalog[r.ID]
		if len(concerns) < 2 {
			t.Errorf("arch role %s: expected ≥2 catalog concerns, got %d", r.ID, len(concerns))
		}
		for i, c := range concerns {
			if c.Area == "" {
				t.Errorf("arch role %s concern[%d]: empty Area", r.ID, i)
			}
			if c.Content == "" {
				t.Errorf("arch role %s concern[%d]: empty Content", r.ID, i)
			}
			if c.Suggestion == "" {
				t.Errorf("arch role %s concern[%d]: empty Suggestion", r.ID, i)
			}
		}
	}
}

// ── False-positive guard ──────────────────────────────────────────────────────

// TestCheckArch_DoesNotBlockPipelineOnWarning — a "warning" arch result must
// NOT set Ready=false on the overall pipeline (warnings are non-blocking).
func TestCheckArch_DoesNotBlockPipelineOnWarning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// No spec → arch returns "warning", but that should not block the pipeline.
	res := RunCheckpoints(root, "non-blocking arch warning", nil)

	archIdx := -1
	for i, cp := range res.Checkpoints {
		if cp.Name == "Arch" {
			archIdx = i
			break
		}
	}
	if archIdx < 0 {
		t.Fatal("Arch checkpoint not found in pipeline results")
	}
	// A warning in arch must not stop the full pipeline.
	if res.Checkpoints[archIdx].Status == "fail" {
		t.Error("arch 'warning' should not be surfaced as 'fail'")
	}
}

// ── Supabase RPC detection ─────────────────────────────────────────────────────

// TestDetectAPIStyle_SupabaseRPC — detectAPIStyle returns "supabase-rpc" when
// the openapi.yaml content contains /rest/v1/rpc/ paths.
func TestDetectAPIStyle_SupabaseRPC(t *testing.T) {
	t.Parallel()
	supabaseContent := `openapi: "3.1.0"
paths:
  /rest/v1/rpc/get_user_profile:
    post:
      summary: Get user profile
`
	if got := detectAPIStyle(supabaseContent); got != "supabase-rpc" {
		t.Errorf("expected supabase-rpc, got %q", got)
	}
}

// TestDetectAPIStyle_REST — detectAPIStyle returns "rest" for standard REST paths.
func TestDetectAPIStyle_REST(t *testing.T) {
	t.Parallel()
	restContent := `openapi: "3.1.0"
paths:
  /api/v1/users:
    post:
      summary: Create user
`
	if got := detectAPIStyle(restContent); got != "rest" {
		t.Errorf("expected rest, got %q", got)
	}
}

// TestDetectAPIStyle_Empty — detectAPIStyle returns "rest" for empty content
// (false-positive guard: no Supabase detection on empty YAML).
func TestDetectAPIStyle_Empty(t *testing.T) {
	t.Parallel()
	if got := detectAPIStyle(""); got != "rest" {
		t.Errorf("expected rest for empty content, got %q", got)
	}
}

// TestOpenapiStub_IncludesSupabaseHint — openapiStub must include a comment
// about the Supabase RPC path alternative so the developer knows to choose.
func TestOpenapiStub_IncludesSupabaseHint(t *testing.T) {
	t.Parallel()
	stub := openapiStub("user notifications")
	if !strings.Contains(stub, "supabase") && !strings.Contains(stub, "Supabase") &&
		!strings.Contains(stub, "rpc") {
		t.Error("openapiStub should mention Supabase RPC as an alternative")
	}
}

// TestArchStub_IncludesSupabaseRPCRow — archStub Section 2 must include a
// Supabase RPC row in the endpoint table so developers see both options.
func TestArchStub_IncludesSupabaseRPCRow(t *testing.T) {
	t.Parallel()
	stub := archStub("send notifications")
	if !strings.Contains(stub, "/rest/v1/rpc/") {
		t.Error("archStub Section 2 should include a /rest/v1/rpc/ example row")
	}
	if !strings.Contains(stub, "Supabase") {
		t.Error("archStub Section 2 should mention Supabase")
	}
}

// TestRoleAPIDesign_FocusAreasMentionSupabase — RoleAPIDesign must include
// Supabase RPC in its focus areas so arch debates surface the choice.
func TestRoleAPIDesign_FocusAreasMentionSupabase(t *testing.T) {
	t.Parallel()
	var apiDesign *Role
	for i := range DefaultArchRoles() {
		r := DefaultArchRoles()[i]
		if r.ID == RoleAPIDesign {
			apiDesign = &r
			break
		}
	}
	if apiDesign == nil {
		t.Fatal("RoleAPIDesign not found in DefaultArchRoles")
	}
	found := false
	for _, fa := range apiDesign.FocusAreas {
		if strings.Contains(strings.ToLower(fa), "supabase") {
			found = true
			break
		}
	}
	if !found {
		t.Error("RoleAPIDesign.FocusAreas should mention Supabase RPC")
	}
}

// TestArchCatalog_APIDesign_SupabaseConcernPresent — archCatalog[RoleAPIDesign]
// must include a concern about Supabase RPC vs REST declaration.
func TestArchCatalog_APIDesign_SupabaseConcernPresent(t *testing.T) {
	t.Parallel()
	concerns := archCatalog[RoleAPIDesign]
	found := false
	for _, c := range concerns {
		if strings.Contains(strings.ToLower(c.Area), "supabase") ||
			strings.Contains(strings.ToLower(c.Content), "supabase") {
			found = true
			break
		}
	}
	if !found {
		t.Error("archCatalog[RoleAPIDesign] should include a Supabase RPC concern")
	}
}

// TestCheckArch_LLM_SupabaseRPCResponse — when the LLM returns an arch document
// with a Supabase-style openapi.yaml block, detectAPIStyle correctly identifies it.
func TestCheckArch_LLM_SupabaseRPCResponse(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := slugify("supabase user profile")
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec: supabase user profile\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	llmResponse := "# Architecture: supabase user profile\n\n## 1. Component Topology\n\nSupabase project.\n\n" +
		"```yaml\nopenapi: \"3.1.0\"\ninfo:\n  title: \"User Profile\"\n  version: \"0.0.0-draft\"\npaths:\n" +
		"  /rest/v1/rpc/get_profile:\n    post:\n      summary: Get profile\n" +
		"components:\n  schemas: {}\n```\n"

	mock := &llmprovider.MockProvider{Response: mockResponse(llmResponse)}
	cp := checkArch(root, "supabase user profile", mockPipe(root, mock))

	if cp.Status != "ok" {
		t.Fatalf("expected ok, got %q: %s", cp.Status, cp.Detail)
	}
	openapiData, err := os.ReadFile(filepath.Join(dir, "openapi.yaml"))
	if err != nil {
		t.Fatalf("openapi.yaml not written: %v", err)
	}
	style := detectAPIStyle(string(openapiData))
	if style != "supabase-rpc" {
		t.Errorf("expected supabase-rpc style detected from extracted openapi.yaml, got %q", style)
	}
}
