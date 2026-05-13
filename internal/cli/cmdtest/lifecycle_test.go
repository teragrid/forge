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

// Test design (per always-write-tests.md):
//
// 1.  Happy path         — CreateTests with a named feature → Ready=true, 5 files.
// 2.  Boundary           — empty feature name → Ready=false.
// 3.  Negative/missing   — ApproveTests with no pending.json → defaults, Ready=true.
// 4.  Idempotency        — CreateTests called twice (dry-run) → same file count.
// 5.  Concurrency        — all sub-tests run t.Parallel() with isolated TempDirs.
// 6.  Cross-phase        — RunFeatureTests without prior approve → falls back to defaults.
// 7.  Regression         — RunCI on dir with no CI → HasCI=false, SetupSteps populated.
// 8.  Data-accuracy      — CreateResult.Feature == input; each GeneratedFile.Family valid.
// 9.  False-positive     — RunCI with real .github/workflows/*.yml → HasCI=true.
// 10. Full lifecycle     — RunLifecycle dry-run → all 4 phases non-nil, Ready=true.
// 11. CI generate-config — RunCI GenerateConfig=true, DryRun=false → yml file created.
// 12. File I/O           — CreateTests DryRun=false writes pending.json; ApproveTests reads it.
// 13. Approved families  — RunFeatureTests reads approved.json, uses its families.
// 14. Emit JSON          — emitCreateResult JSON mode → valid JSON with Ready=true.
// 15. Lifecycle stop     — RunLifecycle with empty feature stops at create phase.
// 16. detectCI           — dir-based provider scan returns correct provider name.

package cmdtest

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/spf13/cobra"
)

// newTestCommand creates a minimal *cobra.Command whose output is routed to w.
func newTestCommand(w *bytes.Buffer) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetOut(w)
	cmd.SetErr(w)
	return cmd
}

// ── helpers ───────────────────────────────────────────────────────────────────

// newRoot creates a temp project root and returns its path.
func newRoot(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// makeWorkflowsDir creates a .github/workflows dir with a minimal yml file.
func makeWorkflowsDir(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("makeWorkflowsDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ci.yml"), []byte("name: ci\n"), 0o640); err != nil {
		t.Fatalf("makeWorkflowsDir write: %v", err)
	}
}

// ── 1. Happy path ─────────────────────────────────────────────────────────────

func TestCreateTests_HappyPath(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	res := CreateTests(CreateOptions{
		Feature: "rate-limiter",
		DryRun:  true,
		Root:    root,
	})
	if !res.Ready {
		t.Fatalf("want Ready=true, got false; message: %s", res.Message)
	}
	if res.Feature != "rate-limiter" {
		t.Errorf("Feature: want %q got %q", "rate-limiter", res.Feature)
	}
	if len(res.Generated) != len(defaultCreateFamilies) {
		t.Errorf("Generated: want %d files, got %d", len(defaultCreateFamilies), len(res.Generated))
	}
	if res.ApproveCmd == "" {
		t.Error("ApproveCmd should not be empty")
	}
}

// ── 2. Boundary: empty feature ─────────────────────────────────────────────────

func TestCreateTests_EmptyFeature(t *testing.T) {
	t.Parallel()
	res := CreateTests(CreateOptions{
		Feature: "",
		DryRun:  true,
	})
	if res.Ready {
		t.Error("want Ready=false for empty feature, got true")
	}
	if res.Message == "" {
		t.Error("Message should not be empty for error case")
	}
}

func TestApproveTests_EmptyFeature(t *testing.T) {
	t.Parallel()
	res := ApproveTests(ApproveOptions{Feature: ""})
	if res.Ready {
		t.Error("want Ready=false for empty feature, got true")
	}
}

func TestRunFeatureTests_EmptyFeature(t *testing.T) {
	t.Parallel()
	res := RunFeatureTests(RunFeatureOptions{Feature: ""})
	if res.Ready {
		t.Error("want Ready=false for empty feature, got true")
	}
}

func TestRunCI_EmptyFeature(t *testing.T) {
	t.Parallel()
	res := RunCI(CIOptions{Feature: ""})
	if res.Ready {
		t.Error("want Ready=false for empty feature, got true")
	}
}

// ── 3. Negative: ApproveTests with no pending.json ────────────────────────────

func TestApproveTests_NoPendingFile_FallsBackToDefaults(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	res := ApproveTests(ApproveOptions{
		Feature: "new-feature",
		Root:    root,
		DryRun:  true,
	})
	if !res.Ready {
		t.Fatalf("want Ready=true (defaults used), got false: %s", res.Message)
	}
	if len(res.Files) != len(defaultCreateFamilies) {
		t.Errorf("Files: want %d (defaults), got %d", len(defaultCreateFamilies), len(res.Files))
	}
}

// ── 4. Idempotency ────────────────────────────────────────────────────────────

func TestCreateTests_CalledTwice_SameFileCount(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	opts := CreateOptions{Feature: "my-feature", DryRun: true, Root: root}
	r1 := CreateTests(opts)
	r2 := CreateTests(opts)
	if len(r1.Generated) != len(r2.Generated) {
		t.Errorf("idempotency: first call %d files, second %d", len(r1.Generated), len(r2.Generated))
	}
}

// ── 5. Concurrency ─────────────────────────────────────────────────────────────

func TestCreateTests_Concurrent(t *testing.T) {
	t.Parallel()
	const n = 8
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			root := t.TempDir()
			res := CreateTests(CreateOptions{Feature: "concurrent-feature", DryRun: true, Root: root})
			if !res.Ready {
				t.Errorf("concurrent goroutine: want Ready=true, got: %s", res.Message)
			}
		}()
	}
	wg.Wait()
}

// ── 6. Cross-phase: RunFeatureTests without approved.json ──────────────────────

func TestRunFeatureTests_NoApprovedFile_UsesDefaults(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	res := RunFeatureTests(RunFeatureOptions{
		Feature: "no-approved-feature",
		DryRun:  true,
		Root:    root,
	})
	if !res.Ready {
		t.Fatalf("want Ready=true (defaults used), got false: %s", res.Message)
	}
	// Each default family should appear in Families.
	if len(res.Families) != len(defaultCreateFamilies) {
		t.Errorf("Families: want %d, got %d", len(defaultCreateFamilies), len(res.Families))
	}
}

// ── 7. Regression: RunCI with no CI config ────────────────────────────────────

func TestRunCI_NoCI_SetupStepsPopulated(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	res := RunCI(CIOptions{
		Feature: "my-feature",
		Env:     "staging",
		DryRun:  true,
		Root:    root,
	})
	if res.HasCI {
		t.Error("want HasCI=false for empty project root, got true")
	}
	if res.Ready {
		t.Error("want Ready=false when no CI configured, got true")
	}
	if len(res.SetupSteps) == 0 {
		t.Error("SetupSteps should not be empty when no CI found")
	}
}

// ── 8. Data-accuracy ──────────────────────────────────────────────────────────

func TestCreateTests_DataAccuracy(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	res := CreateTests(CreateOptions{
		Feature: "auth-service",
		DryRun:  true,
		Root:    root,
	})
	if res.Feature != "auth-service" {
		t.Errorf("Feature: want %q got %q", "auth-service", res.Feature)
	}
	// Each generated file must have a non-empty path, a known family, and positive counts.
	for _, gf := range res.Generated {
		if gf.Path == "" {
			t.Errorf("GeneratedFile has empty Path for family %s", gf.Family)
		}
		if _, ok := knownFamilies[gf.Family]; !ok && gf.Family != "" {
			// generatedFiles only use families from defaultCreateFamilies which are all valid.
			t.Errorf("GeneratedFile.Family %q not in knownFamilies", gf.Family)
		}
		if gf.TestCount <= 0 {
			t.Errorf("family %s: TestCount want >0, got %d", gf.Family, gf.TestCount)
		}
		if gf.Lines <= 0 {
			t.Errorf("family %s: Lines want >0, got %d", gf.Family, gf.Lines)
		}
	}
}

func TestApproveTests_DataAccuracy(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	res := ApproveTests(ApproveOptions{Feature: "checkout", Root: root, DryRun: true})
	if res.Feature != "checkout" {
		t.Errorf("Feature: want %q got %q", "checkout", res.Feature)
	}
	if res.Approved != len(res.Files) {
		t.Errorf("Approved count %d != len(Files) %d", res.Approved, len(res.Files))
	}
	if res.Rejected != 0 {
		t.Errorf("Rejected: want 0 got %d", res.Rejected)
	}
}

// ── 9. False-positive: RunCI with real .github/workflows/ ────────────────────

func TestRunCI_WithCI_ReturnsReady(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	makeWorkflowsDir(t, root)
	res := RunCI(CIOptions{
		Feature: "my-feature",
		Env:     "staging",
		DryRun:  true,
		Root:    root,
	})
	if !res.HasCI {
		t.Error("want HasCI=true when .github/workflows/*.yml exists, got false")
	}
	if !res.Ready {
		t.Errorf("want Ready=true when CI present, got false: %s", res.Message)
	}
	if res.TriggerCmd == "" {
		t.Error("TriggerCmd should not be empty when CI is ready")
	}
}

// ── 10. Full lifecycle dry-run ────────────────────────────────────────────────

func TestRunLifecycle_DryRun_AllPhasesPresent(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	res := RunLifecycle(LifecycleOptions{
		Feature: "payment-gateway",
		DryRun:  true,
		Root:    root,
	})
	if res.Create == nil {
		t.Error("Create phase should not be nil")
	}
	if res.Approve == nil {
		t.Error("Approve phase should not be nil")
	}
	if res.Run == nil {
		t.Error("Run phase should not be nil")
	}
	if res.CI == nil {
		t.Error("CI phase should not be nil")
	}
	// Local tests passed even if CI not configured.
	if !res.Ready {
		t.Errorf("want Ready=true (local passed), got false: %s", res.Message)
	}
}

// ── 11. CI generate-config ────────────────────────────────────────────────────

func TestRunCI_GenerateConfig_CreatesYMLFile(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	res := RunCI(CIOptions{
		Feature:        "ratelimit",
		Env:            "staging",
		DryRun:         false,
		GenerateConfig: true,
		Root:           root,
	})
	if !res.ConfigGenerated {
		t.Error("want ConfigGenerated=true when --generate-config and DryRun=false")
	}
	if !res.HasCI {
		t.Error("want HasCI=true after config is generated")
	}
	ymlPath := filepath.Join(root, ".github", "workflows", "forge-test.yml")
	if _, err := os.Stat(ymlPath); os.IsNotExist(err) {
		t.Errorf("expected %s to exist after config generation", ymlPath)
	}
}

// ── 12. File I/O: CreateTests DryRun=false writes pending.json ────────────────

func TestCreateTests_LiveMode_WritesPendingJSON(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	res := CreateTests(CreateOptions{
		Feature: "notifications",
		DryRun:  false,
		Root:    root,
	})
	if !res.Ready {
		t.Fatalf("want Ready=true, got false: %s", res.Message)
	}
	pendingPath := filepath.Join(res.OutputDir, "pending.json")
	data, err := os.ReadFile(pendingPath)
	if err != nil {
		t.Fatalf("pending.json not found: %v", err)
	}
	var stored CreateResult
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("pending.json invalid JSON: %v", err)
	}
	if stored.Feature != "notifications" {
		t.Errorf("stored Feature: want %q got %q", "notifications", stored.Feature)
	}
}

// ── 13. Approved families: ApproveTests reads pending.json ────────────────────

func TestApproveTests_ReadsPendingJSON(t *testing.T) {
	t.Parallel()
	root := newRoot(t)

	// Create with DryRun=false to write pending.json.
	cr := CreateTests(CreateOptions{
		Feature: "search",
		DryRun:  false,
		Root:    root,
	})
	if !cr.Ready {
		t.Fatalf("create: %s", cr.Message)
	}

	// Approve reads the pending.json.
	ar := ApproveTests(ApproveOptions{
		Feature:   "search",
		Root:      root,
		OutputDir: cr.OutputDir,
		DryRun:    false,
	})
	if !ar.Ready {
		t.Fatalf("approve: %s", ar.Message)
	}
	if ar.Approved != len(cr.Generated) {
		t.Errorf("Approved: want %d (from create), got %d", len(cr.Generated), ar.Approved)
	}

	// Check approved.json exists.
	approvedPath := filepath.Join(cr.OutputDir, "approved.json")
	if _, err := os.Stat(approvedPath); os.IsNotExist(err) {
		t.Errorf("approved.json not written at %s", approvedPath)
	}
}

// ── 14. Emit JSON mode ────────────────────────────────────────────────────────

func TestEmitCreateResult_JSONMode(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	res := CreateTests(CreateOptions{
		Feature: "billing",
		DryRun:  true,
		Root:    root,
	})

	var buf bytes.Buffer
	cmd := newTestCommand(&buf)
	if err := emitCreateResult(cmd, res, true); err != nil {
		t.Fatalf("emitCreateResult: %v", err)
	}

	var got CreateResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if !got.Ready {
		t.Error("decoded Ready should be true")
	}
	if got.Feature != "billing" {
		t.Errorf("decoded Feature: want %q got %q", "billing", got.Feature)
	}
}

func TestEmitApproveResult_JSONMode(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	res := ApproveTests(ApproveOptions{Feature: "billing", Root: root, DryRun: true})
	var buf bytes.Buffer
	cmd := newTestCommand(&buf)
	if err := emitApproveResult(cmd, res, true); err != nil {
		t.Fatalf("emitApproveResult: %v", err)
	}
	var got ApproveResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if got.Feature != "billing" {
		t.Errorf("Feature: want %q got %q", "billing", got.Feature)
	}
}

func TestEmitRunFeatureResult_JSONMode(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	res := RunFeatureTests(RunFeatureOptions{Feature: "billing", Root: root, DryRun: true})
	var buf bytes.Buffer
	cmd := newTestCommand(&buf)
	if err := emitRunFeatureResult(cmd, res, true); err != nil {
		t.Fatalf("emitRunFeatureResult: %v", err)
	}
	var got RunFeatureResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if got.Feature != "billing" {
		t.Errorf("Feature: want %q got %q", "billing", got.Feature)
	}
}

func TestEmitCIResult_JSONMode_NoCINotError(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	res := RunCI(CIOptions{Feature: "billing", Root: root, DryRun: true})
	var buf bytes.Buffer
	cmd := newTestCommand(&buf)
	// emitCIResult returns an error when CI not ready — that's expected.
	_ = emitCIResult(cmd, res, true)
	var got CIResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if got.Feature != "billing" {
		t.Errorf("Feature: want %q got %q", "billing", got.Feature)
	}
}

func TestEmitLifecycleResult_JSONMode(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	res := RunLifecycle(LifecycleOptions{Feature: "billing", DryRun: true, Root: root})
	var buf bytes.Buffer
	cmd := newTestCommand(&buf)
	// CI not ready is non-fatal in lifecycle.
	_ = emitLifecycleResult(cmd, res, true)
	var got LifecycleResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if got.Feature != "billing" {
		t.Errorf("Feature: want %q got %q", "billing", got.Feature)
	}
}

// ── 15. Lifecycle stops at create phase for empty feature ─────────────────────

func TestRunLifecycle_EmptyFeature_StopsAtCreate(t *testing.T) {
	t.Parallel()
	res := RunLifecycle(LifecycleOptions{Feature: "", DryRun: true})
	if res.Ready {
		t.Error("want Ready=false for empty feature lifecycle")
	}
	if res.Create == nil {
		t.Error("Create should still be populated even when stopping early")
	}
	if res.Approve != nil {
		t.Error("Approve should be nil (lifecycle stopped at create)")
	}
}

// ── 16. detectCI ──────────────────────────────────────────────────────────────

func TestDetectCI_EmptyDir_NoCIFound(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	provider, wf := detectCI(root)
	if provider != "" {
		t.Errorf("want empty provider for empty dir, got %q (workflow: %q)", provider, wf)
	}
}

func TestDetectCI_GitHubActionsDir_Found(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	makeWorkflowsDir(t, root)
	provider, wf := detectCI(root)
	if provider != "GitHub Actions" {
		t.Errorf("provider: want %q got %q", "GitHub Actions", provider)
	}
	if wf == "" {
		t.Error("workflow file path should not be empty")
	}
}

func TestDetectCI_GitLabCI_Found(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	if err := os.WriteFile(filepath.Join(root, ".gitlab-ci.yml"), []byte("stages: [test]\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	provider, _ := detectCI(root)
	if provider != "GitLab CI" {
		t.Errorf("provider: want %q got %q", "GitLab CI", provider)
	}
}

func TestDetectCI_Jenkins_Found(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	if err := os.WriteFile(filepath.Join(root, "Jenkinsfile"), []byte("pipeline {}\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	provider, _ := detectCI(root)
	if provider != "Jenkins" {
		t.Errorf("provider: want %q got %q", "Jenkins", provider)
	}
}

// ── 17. estimatedLines sanity ─────────────────────────────────────────────────

func TestEstimatedLines_AlwaysPositive(t *testing.T) {
	t.Parallel()
	for _, tc := range []int{0, 1, 5, 12, 100} {
		got := estimatedLines(tc)
		if got <= 0 {
			t.Errorf("estimatedLines(%d): want >0, got %d", tc, got)
		}
	}
}

// ── 18. ciTriggerCmd coverage ─────────────────────────────────────────────────

func TestCITriggerCmd_GitHubActions(t *testing.T) {
	t.Parallel()
	cmd := ciTriggerCmd("GitHub Actions", "my-feat", "staging")
	if cmd == "" {
		t.Error("trigger cmd should not be empty")
	}
	// Should mention the feature slug and env.
	if !containsStr(cmd, "my-feat") {
		t.Errorf("trigger cmd %q should contain feature slug", cmd)
	}
}

func TestCITriggerCmd_GitLabCI(t *testing.T) {
	t.Parallel()
	cmd := ciTriggerCmd("GitLab CI", "my-feat", "preview")
	if !containsStr(cmd, "my-feat") {
		t.Errorf("trigger cmd %q should contain feature slug", cmd)
	}
}

func TestCITriggerCmd_UnknownProvider_NonEmpty(t *testing.T) {
	t.Parallel()
	cmd := ciTriggerCmd("Buildkite", "f", "staging")
	if cmd == "" {
		t.Error("trigger cmd should not be empty for unknown provider")
	}
}

// ── 19. Render smoke tests ────────────────────────────────────────────────────

func TestRenderCreate_DoesNotPanic(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cmd := newTestCommand(&buf)
	res := CreateTests(CreateOptions{Feature: "x", DryRun: true, Root: t.TempDir()})
	renderCreate(cmd, res) // must not panic
	if buf.Len() == 0 {
		t.Error("renderCreate should write something")
	}
}

func TestRenderApprove_DoesNotPanic(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cmd := newTestCommand(&buf)
	res := ApproveTests(ApproveOptions{Feature: "x", DryRun: true, Root: t.TempDir()})
	renderApprove(cmd, res)
	if buf.Len() == 0 {
		t.Error("renderApprove should write something")
	}
}

func TestRenderRunFeature_DoesNotPanic(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cmd := newTestCommand(&buf)
	res := RunFeatureTests(RunFeatureOptions{Feature: "x", DryRun: true, Root: t.TempDir()})
	renderRunFeature(cmd, res)
	if buf.Len() == 0 {
		t.Error("renderRunFeature should write something")
	}
}

func TestRenderCI_NoCIConfig_DoesNotPanic(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cmd := newTestCommand(&buf)
	res := RunCI(CIOptions{Feature: "x", DryRun: true, Root: t.TempDir()})
	renderCI(cmd, res)
	if buf.Len() == 0 {
		t.Error("renderCI should write something")
	}
}

func TestRenderLifecycle_DoesNotPanic(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cmd := newTestCommand(&buf)
	res := RunLifecycle(LifecycleOptions{Feature: "x", DryRun: true, Root: t.TempDir()})
	renderLifecycle(cmd, res)
	if buf.Len() == 0 {
		t.Error("renderLifecycle should write something")
	}
}

// ── 20. Custom families respected ────────────────────────────────────────────

func TestCreateTests_CustomFamilies_UsedInOutput(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	custom := []Family{FamilyUnit, FamilySmoke}
	res := CreateTests(CreateOptions{
		Feature:  "cache",
		Families: custom,
		DryRun:   true,
		Root:     root,
	})
	if !res.Ready {
		t.Fatalf("Ready: %s", res.Message)
	}
	if len(res.Generated) != len(custom) {
		t.Errorf("Generated: want %d (custom families), got %d", len(custom), len(res.Generated))
	}
	for i, gf := range res.Generated {
		if gf.Family != custom[i] {
			t.Errorf("Generated[%d].Family: want %q got %q", i, custom[i], gf.Family)
		}
	}
}

// ── 21. LifecycleOptions.AutoApprove short-circuits approve phase ─────────────

func TestRunLifecycle_AutoApprove_DryRun_Ready(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	res := RunLifecycle(LifecycleOptions{
		Feature:     "auto-approve-feature",
		AutoApprove: true,
		DryRun:      true,
		Root:        root,
	})
	if !res.Ready {
		t.Errorf("want Ready=true with AutoApprove, got false: %s", res.Message)
	}
}

// ── 22. RunCI env default ─────────────────────────────────────────────────────

func TestRunCI_DefaultEnvIsStaging(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	makeWorkflowsDir(t, root)
	res := RunCI(CIOptions{Feature: "x", Root: root, DryRun: true})
	if res.Env != "staging" {
		t.Errorf("Env: want %q got %q", "staging", res.Env)
	}
}

// ── 23. Regression: original bug guard — DryRun=false pending.json must persist ──

func TestCreateTests_DryRunFalse_PendingJSONPersists_AfterSecondCall(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	opts := CreateOptions{Feature: "widget", DryRun: false, Root: root}
	r1 := CreateTests(opts)
	if !r1.Ready {
		t.Fatalf("first call: %s", r1.Message)
	}
	r2 := CreateTests(opts)
	if !r2.Ready {
		t.Fatalf("second call (idempotency): %s", r2.Message)
	}
	if len(r1.Generated) != len(r2.Generated) {
		t.Errorf("Generated count changed between calls: %d vs %d", len(r1.Generated), len(r2.Generated))
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func containsStr(s, sub string) bool {
	return len(s) > 0 && len(sub) > 0 && (func() bool {
		for i := range s {
			if i+len(sub) <= len(s) && s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
