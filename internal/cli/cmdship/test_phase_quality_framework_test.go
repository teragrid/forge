// test_phase_quality_framework_test.go
// RFC-005 §6 — Test Phase Quality Framework
// TDD state: RED (stubs intentionally incomplete — filled at Code checkpoint)
//
// 9-dimension test design:
// D1 Happy path     — detect Go/Python/TS/Java; write correct stubs; traceability written
// D2 Boundary       — empty root, zero ACs, exactly 500-entry cap, Go go-version boundary (1.17 vs 1.18)
// D3 Negative       — missing root, malformed spec.yml, unknown stack → graceful empty result
// D4 Idempotency    — WriteTraceability called twice returns same result
// D5 Concurrency    — parallel detectTestFramework for two roots; no data race
// D6 AuthZ/Path     — slug "../escape" rejected by fssandbox before any write
// D7 Regression     — Go project must NOT produce .test.ts (G10 guard)
//                   — bug-fix feature description triggers D7 mandatory stubs
// D8 Data-accuracy  — traceability.yaml round-trip: write → read → assert fields exact
// D9 False-positive — gate_status WARNING (not BLOCK) when composite ≥ 6.5 but dims missing
//                   — BLOCK only fires when composite < 6.5 OR high-weight dim below threshold

package cmdship

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/teragrid/forge/internal/llmprovider"
)

// ── AC-001 + AC-002: TestFrameworkContext & detectTestFramework ───────────────

func TestDetectTestFramework_Go_HappyPath(t *testing.T) {
	root := t.TempDir()
	must(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\ngo 1.22\n"), 0o600))

	fw := detectTestFramework(root)

	if fw.Language != "go" {
		t.Errorf("Language: want go, got %q", fw.Language)
	}
	if fw.TestRunner != "go test" {
		t.Errorf("TestRunner: want 'go test', got %q", fw.TestRunner)
	}
	if fw.CoverageCmd == "" {
		t.Error("CoverageCmd must not be empty for Go project")
	}
	if !fw.FuzzSupport {
		t.Error("FuzzSupport must be true for go 1.22")
	}
}

func TestDetectTestFramework_Go_Pre118_NoFuzz(t *testing.T) {
	// Boundary: go 1.17 — fuzz was introduced in 1.18
	root := t.TempDir()
	must(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\ngo 1.17\n"), 0o600))

	fw := detectTestFramework(root)

	if fw.FuzzSupport {
		t.Error("FuzzSupport must be false for go 1.17 (fuzz requires 1.18+)")
	}
}

func TestDetectTestFramework_Go_118_FuzzBoundary(t *testing.T) {
	// Boundary: go 1.18 — exactly the fuzz introduction version
	root := t.TempDir()
	must(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\ngo 1.18\n"), 0o600))

	fw := detectTestFramework(root)

	if !fw.FuzzSupport {
		t.Error("FuzzSupport must be true for go exactly 1.18")
	}
}

func TestDetectTestFramework_Python_WithHypothesis(t *testing.T) {
	root := t.TempDir()
	must(t, os.WriteFile(filepath.Join(root, "requirements.txt"), []byte("pytest==8.0.0\nhypothesis==6.100.0\n"), 0o600))

	fw := detectTestFramework(root)

	if fw.Language != "python" {
		t.Errorf("Language: want python, got %q", fw.Language)
	}
	if fw.TestRunner != "pytest" {
		t.Errorf("TestRunner: want pytest, got %q", fw.TestRunner)
	}
	if !fw.FuzzSupport {
		t.Error("FuzzSupport must be true when hypothesis is in requirements.txt")
	}
}

func TestDetectTestFramework_TypeScript_Jest(t *testing.T) {
	root := t.TempDir()
	pkg := `{"name":"app","devDependencies":{"jest":"^29.0.0","@types/jest":"^29.0.0"}}`
	must(t, os.WriteFile(filepath.Join(root, "package.json"), []byte(pkg), 0o600))

	fw := detectTestFramework(root)

	if fw.Language != "typescript" {
		t.Errorf("Language: want typescript, got %q", fw.Language)
	}
	if fw.TestRunner != "jest" {
		t.Errorf("TestRunner: want jest, got %q", fw.TestRunner)
	}
}

func TestDetectTestFramework_Java_Maven(t *testing.T) {
	root := t.TempDir()
	must(t, os.WriteFile(filepath.Join(root, "pom.xml"), []byte("<project/>"), 0o600))

	fw := detectTestFramework(root)

	if fw.Language != "java" {
		t.Errorf("Language: want java, got %q", fw.Language)
	}
	if fw.TestRunner != "junit" {
		t.Errorf("TestRunner: want junit, got %q", fw.TestRunner)
	}
}

func TestDetectTestFramework_EmptyRoot_ReturnsEmpty(t *testing.T) {
	// Boundary: empty project directory — unknown stack
	root := t.TempDir()

	fw := detectTestFramework(root)

	if fw.Language != "" {
		t.Errorf("Language should be empty for unknown stack, got %q", fw.Language)
	}
	if fw.TestRunner != "" {
		t.Errorf("TestRunner should be empty for unknown stack, got %q", fw.TestRunner)
	}
}

func TestDetectTestFramework_MissingRoot_DoesNotPanic(_ *testing.T) {
	// Negative: root does not exist
	fw := detectTestFramework("/nonexistent/path/that/cannot/exist/xyz123")
	// Must not panic; returns zero-value context
	_ = fw
}

func TestDetectTestFramework_ExistingTests_CollectsUpTo5(t *testing.T) {
	root := t.TempDir()
	must(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\ngo 1.22\n"), 0o600))
	// Create 7 test files — only 5 should be sampled
	for i := range 7 {
		name := fmt.Sprintf("feature%d_test.go", i)
		must(t, os.WriteFile(filepath.Join(root, name), []byte("package x\n"), 0o600))
	}

	fw := detectTestFramework(root)

	if len(fw.ExistingTests) > 5 {
		t.Errorf("ExistingTests capped at 5, got %d", len(fw.ExistingTests))
	}
}

func TestDetectTestFramework_IntegTestDir_Detected(t *testing.T) {
	root := t.TempDir()
	must(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\ngo 1.22\n"), 0o600))
	must(t, os.MkdirAll(filepath.Join(root, "tests", "integration"), 0o755))

	fw := detectTestFramework(root)

	if fw.IntegTestDir == "" {
		t.Error("IntegTestDir should be detected when tests/integration/ exists")
	}
}

func TestDetectTestFramework_Concurrency_NoRace(t *testing.T) {
	// D5: two goroutines each scanning a different root — no data race
	root1 := t.TempDir()
	root2 := t.TempDir()
	must(t, os.WriteFile(filepath.Join(root1, "go.mod"), []byte("module x\ngo 1.22\n"), 0o600))
	must(t, os.WriteFile(filepath.Join(root2, "requirements.txt"), []byte("pytest==8.0.0\n"), 0o600))

	var wg sync.WaitGroup
	var fw1, fw2 TestFrameworkContext
	wg.Add(2)
	go func() { defer wg.Done(); fw1 = detectTestFramework(root1) }()
	go func() { defer wg.Done(); fw2 = detectTestFramework(root2) }()
	wg.Wait()

	if fw1.Language != "go" {
		t.Errorf("fw1.Language want go, got %q", fw1.Language)
	}
	if fw2.Language != "python" {
		t.Errorf("fw2.Language want python, got %q", fw2.Language)
	}
}

// ── AC-003 + AC-006: Framework-aware stub generation (G10 regression) ─────────

func TestWriteTestArtifactsWithContext_Go_GeneratesGoStubs(t *testing.T) {
	root, slug := testRoot(t, "go")

	fw := detectTestFramework(root)
	paths := writeTestArtifactsWithContext(root, slug, "my feature", "", fw, nil)

	if paths.GoTest == "" {
		t.Fatal("GoTest path must be set for Go project")
	}
	if _, err := os.Stat(paths.GoTest); err != nil {
		t.Errorf("GoTest file not written: %v", err)
	}
	content, _ := os.ReadFile(paths.GoTest)
	if !strings.Contains(string(content), "func Test") {
		t.Error("GoTest must contain at least one func Test...")
	}
}

func TestWriteTestArtifactsWithContext_Go_NoTypeScriptStubs(t *testing.T) {
	// D7 Regression guard: G10 — TS stubs MUST NOT be generated for Go projects
	root, slug := testRoot(t, "go")

	fw := detectTestFramework(root)
	paths := writeTestArtifactsWithContext(root, slug, "my feature", "", fw, nil)

	if paths.UnitTest != "" {
		if _, err := os.Stat(paths.UnitTest); err == nil {
			t.Error("TypeScript .test.ts must NOT be generated for a Go project")
		}
	}
	if paths.IntegrationTest != "" {
		if _, err := os.Stat(paths.IntegrationTest); err == nil {
			t.Error("TypeScript .integration.test.ts must NOT be generated for a Go project")
		}
	}
}

func TestWriteTestArtifactsWithContext_TypeScript_NoGoStubs(t *testing.T) {
	// D7 Regression guard: Go stubs MUST NOT be generated for TS projects
	root, slug := testRoot(t, "ts")

	fw := detectTestFramework(root)
	paths := writeTestArtifactsWithContext(root, slug, "my feature", "", fw, nil)

	if paths.UnitTest == "" {
		t.Fatal("UnitTest path must be set for TypeScript project")
	}
	if _, err := os.Stat(paths.UnitTest); err != nil {
		t.Errorf("UnitTest .test.ts not written: %v", err)
	}
	if paths.GoTest != "" {
		if _, err := os.Stat(paths.GoTest); err == nil {
			t.Error("Go _test.go must NOT be generated for TypeScript project")
		}
	}
}

// TestWriteTestArtifactsWithContext_RLSPromptNamesDetectedFramework guards
// against the exact defect found dogfooding on ai-marketing-platfrom
// (2026-07-24): the RLS stub prompt named no test framework at all (unlike
// the unit/integration prompts, which hardcoded "Jest" as a literal string
// instead of using the already-detected fw.TestRunner) — so on a real Jest
// project, the LLM defaulted to inventing `import ... from "vitest"` in
// rls.test.ts. All three prompts must explicitly name the *detected*
// framework, not a hardcoded one.
func TestWriteTestArtifactsWithContext_RLSPromptNamesDetectedFramework(t *testing.T) {
	root, slug := testRoot(t, "ts")
	fw := detectTestFramework(root)
	if fw.TestRunner != "jest" {
		t.Fatalf("test setup: expected jest detection, got %q", fw.TestRunner)
	}

	var rlsSystemPrompt, unitSystemPrompt, integSystemPrompt string
	mock := &llmprovider.MockProvider{
		Fn: func(req *llmprovider.Request) (*llmprovider.Response, error) {
			switch {
			case strings.Contains(req.UserPrompt, "RLS test stubs"):
				rlsSystemPrompt = req.SystemPrompt
				return &llmprovider.Response{Content: "// rls stub\n"}, nil
			case strings.Contains(req.UserPrompt, "integration test stubs"):
				integSystemPrompt = req.SystemPrompt
				return &llmprovider.Response{Content: "// integration stub\n"}, nil
			default:
				unitSystemPrompt = req.SystemPrompt
				return &llmprovider.Response{Content: "// unit stub\n"}, nil
			}
		},
	}

	writeTestArtifactsWithContext(root, slug, "my feature", "", fw, mockPipe(root, mock))

	for name, prompt := range map[string]string{
		"RLS": rlsSystemPrompt, "unit": unitSystemPrompt, "integration": integSystemPrompt,
	} {
		if !strings.Contains(strings.ToLower(prompt), "jest") {
			t.Errorf("%s prompt must explicitly name the detected framework (jest); got: %q", name, prompt)
		}
	}
}

func TestWriteTestArtifactsWithContext_Idempotent(t *testing.T) {
	// D4: calling twice must produce the same paths and not corrupt files
	root, slug := testRoot(t, "go")
	fw := detectTestFramework(root)

	p1 := writeTestArtifactsWithContext(root, slug, "feature", "", fw, nil)
	p2 := writeTestArtifactsWithContext(root, slug, "feature", "", fw, nil)

	if p1.GoTest != p2.GoTest {
		t.Errorf("idempotency: GoTest paths differ: %q vs %q", p1.GoTest, p2.GoTest)
	}
	content, err := os.ReadFile(p2.GoTest)
	if err != nil {
		t.Fatalf("second call file unreadable: %v", err)
	}
	if len(content) == 0 {
		t.Error("second call must not overwrite with empty file")
	}
}

// ── AC-004: Bug-fix regression guard ──────────────────────────────────────────

func TestIsBugFix_DetectsFixSignal(t *testing.T) {
	cases := []struct {
		desc    string
		feature string
		want    bool
	}{
		{"explicit fix", "fix nil pointer in invoice parser", true},
		{"bug keyword", "bug: wrong output for zero amount", true},
		{"regression", "regression: auth bypass on expired token", true},
		{"normal feature", "add PDF export endpoint", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			got := IsBugFix(tc.feature)
			if got != tc.want {
				t.Errorf("IsBugFix(%q) = %v, want %v", tc.feature, got, tc.want)
			}
		})
	}
}

func TestWriteTestArtifactsWithContext_BugFix_HasRegressionStubs(t *testing.T) {
	// D7: bug-fix feature must produce two Regression-labeled stubs
	root, slug := testRoot(t, "go")
	fw := detectTestFramework(root)

	paths := writeTestArtifactsWithContext(root, slug, "fix nil pointer in parser", "", fw, nil)

	content, err := os.ReadFile(paths.GoTest)
	if err != nil {
		t.Fatalf("GoTest not written: %v", err)
	}
	body := string(content)
	if !strings.Contains(body, "Regression") {
		t.Error("bug-fix feature must include // Regression: labeled test stubs")
	}
}

// ── AC-007 + AC-008: 9-dimension scoring ──────────────────────────────────────

func TestComputeCompositeScore_AllPerfect_GivesPass(t *testing.T) {
	// D1 happy path: all 9 dimensions score 10 → PASS
	scores := allScores(10.0)
	result := ComputeCompositeScore(scores, "T1", false, nil)

	if result.GateStatus != "PASS" {
		t.Errorf("all-10 scores: want PASS, got %q", result.GateStatus)
	}
	if result.CompositeScore < 9.9 {
		t.Errorf("all-10 scores: composite should be ~10, got %f", result.CompositeScore)
	}
}

func TestComputeCompositeScore_CompositeBelow65_GivesBlock(t *testing.T) {
	// D3 negative: composite < 6.5 must BLOCK
	scores := allScores(5.0)
	result := ComputeCompositeScore(scores, "T1", false, nil)

	if result.GateStatus != "BLOCK" {
		t.Errorf("all-5 scores: want BLOCK, got %q", result.GateStatus)
	}
	if len(result.BlockingReasons) == 0 {
		t.Error("BLOCK result must include at least one blocking reason")
	}
}

func TestComputeCompositeScore_HighWeightDimBelowThreshold_GivesBlock(t *testing.T) {
	// D3 negative: D6 (weight 2.0, threshold 8) scoring 7 must BLOCK
	// even if composite is ≥ 6.5
	scores := allScores(8.0)
	scores[int(D6AuthZ)-1].Score = 7.0 // below D6 threshold of 8
	result := ComputeCompositeScore(scores, "T1", false, nil)

	if result.GateStatus != "BLOCK" {
		t.Errorf("D6=7 (threshold 8): want BLOCK, got %q (composite=%.2f)", result.GateStatus, result.CompositeScore)
	}
}

func TestComputeCompositeScore_MissingDimensions_GivesWarning(t *testing.T) {
	// D9 false-positive guard: composite ≥ 6.5 but D4 and D5 not covered → WARNING (not BLOCK)
	// Use score 8.0 so D6 (threshold 8) is exactly at threshold and does not block.
	scores := allScores(8.0)
	scores[int(D4Idempotency)-1].Covered = false
	scores[int(D5Concurrency)-1].Covered = false
	result := ComputeCompositeScore(scores, "T1", false, nil)

	if result.GateStatus == "BLOCK" {
		t.Errorf("missing D4/D5 with good composite should be WARNING, not BLOCK")
	}
	if result.GateStatus != "WARNING" {
		t.Errorf("want WARNING for missing dims, got %q", result.GateStatus)
	}
	if len(result.MissingDims) == 0 {
		t.Error("MissingDims must list D4 and D5")
	}
}

func TestComputeCompositeScore_T0_WaivesD5D6D8(t *testing.T) {
	// D2 boundary + D9 false-positive: T0 waives D5/D6/D8
	scores := allScores(7.0)
	scores[int(D5Concurrency)-1].Score = 0.0
	scores[int(D6AuthZ)-1].Score = 0.0
	scores[int(D8DataAccuracy)-1].Score = 0.0
	waivers := []Dimension{D5Concurrency, D6AuthZ, D8DataAccuracy}

	result := ComputeCompositeScore(scores, "T0", false, waivers)

	if result.GateStatus == "BLOCK" {
		t.Errorf("T0 with D5/D6/D8 waived: must not BLOCK, got %q", result.GateStatus)
	}
}

func TestComputeCompositeScore_T2_RaisesD6Threshold(t *testing.T) {
	// D2 boundary: T2 tier raises D6 threshold to ≥9; D6=8 should BLOCK
	scores := allScores(9.0)
	scores[int(D6AuthZ)-1].Score = 8.0 // at standard threshold but below T2 threshold of 9
	result := ComputeCompositeScore(scores, "T2", false, nil)

	if result.GateStatus != "BLOCK" {
		t.Errorf("T2 tier, D6=8 (T2 threshold 9): want BLOCK, got %q", result.GateStatus)
	}
}

func TestComputeCompositeScore_BugFix_D7MandatoryAt8(t *testing.T) {
	// D7: bug-fix mode forces D7 threshold to 8 regardless of tier
	scores := allScores(8.0)
	scores[int(D7Regression)-1].Score = 7.0 // standard threshold 7 but bug-fix requires 8
	result := ComputeCompositeScore(scores, "T1", true /* isBugFix */, nil)

	if result.GateStatus != "BLOCK" {
		t.Errorf("bug-fix with D7=7 (bug-fix threshold 8): want BLOCK, got %q", result.GateStatus)
	}
}

func TestComputeCompositeScore_Idempotent(t *testing.T) {
	// D4: same inputs produce identical output
	scores := allScores(7.5)
	r1 := ComputeCompositeScore(scores, "T1", false, nil)
	r2 := ComputeCompositeScore(scores, "T1", false, nil)

	if r1.CompositeScore != r2.CompositeScore {
		t.Errorf("idempotency: composite %.4f != %.4f", r1.CompositeScore, r2.CompositeScore)
	}
	if r1.GateStatus != r2.GateStatus {
		t.Errorf("idempotency: gate %q != %q", r1.GateStatus, r2.GateStatus)
	}
}

// ── AC-010: TraceabilityMatrix write / read (RFC §6.5 format) ─────────────────

func TestWriteTraceability_FullMatrix_RoundTrip(t *testing.T) {
	// D8: write full matrix, read back, assert every field exact
	specsDir := t.TempDir()
	slug := "invoice-pdf"
	must(t, os.MkdirAll(filepath.Join(specsDir, slug), 0o755))

	generated := time.Now().UTC().Format(time.RFC3339)
	hash := sha256.Sum256([]byte("spec content"))
	specVer := fmt.Sprintf("sha256:%x", hash)

	m := TraceabilityMatrix{
		Feature:     slug,
		Generated:   generated,
		SpecVersion: specVer,
		Matrix: []ACEntry{
			{
				ACID:   "AC-001",
				ACText: "User can export invoice to PDF",
				Tests: []TestEntry{
					{ID: "T-001", Name: "TestInvoiceExport_HappyPath", Type: "integration", Dimension: "D1", File: "internal/invoice/export_test.go"},
					{ID: "T-002", Name: "TestInvoiceExport_Unauthorized", Type: "integration", Dimension: "D6", File: "internal/invoice/export_test.go"},
				},
			},
		},
		CoverageSummary: CoverageSummary{
			TotalACs:          1,
			ACsWithTests:      1,
			TotalTests:        2,
			DimensionCoverage: map[string]bool{"D1": true, "D6": true},
			CompositeScore:    7.8,
			MissingDimensions: []string{"D2", "D3", "D4", "D5", "D7", "D8", "D9"},
			WaivedDimensions:  []string{},
			GateStatus:        "WARNING",
		},
	}

	if err := WriteTraceability(specsDir, slug, m); err != nil {
		t.Fatalf("WriteTraceability: %v", err)
	}
	got, err := ReadTraceability(specsDir, slug)
	if err != nil {
		t.Fatalf("ReadTraceability: %v", err)
	}

	if got.Feature != slug {
		t.Errorf("Feature: want %q, got %q", slug, got.Feature)
	}
	if got.SpecVersion != specVer {
		t.Errorf("SpecVersion: want %q, got %q", specVer, got.SpecVersion)
	}
	if len(got.Matrix) != 1 {
		t.Errorf("Matrix len: want 1, got %d", len(got.Matrix))
	}
	if got.Matrix[0].ACID != "AC-001" {
		t.Errorf("Matrix[0].ACID: want AC-001, got %q", got.Matrix[0].ACID)
	}
	if len(got.Matrix[0].Tests) != 2 {
		t.Errorf("Matrix[0].Tests len: want 2, got %d", len(got.Matrix[0].Tests))
	}
	if got.CoverageSummary.CompositeScore != 7.8 {
		t.Errorf("CompositeScore: want 7.8, got %f", got.CoverageSummary.CompositeScore)
	}
	if got.CoverageSummary.GateStatus != "WARNING" {
		t.Errorf("GateStatus: want WARNING, got %q", got.CoverageSummary.GateStatus)
	}
}

func TestWriteTraceability_Idempotent(t *testing.T) {
	// D4: WriteTraceability called twice must not corrupt the file
	specsDir := t.TempDir()
	slug := "idem"
	must(t, os.MkdirAll(filepath.Join(specsDir, slug), 0o755))

	m := TraceabilityMatrix{Feature: slug, Generated: "2026-06-02T00:00:00Z", Matrix: nil,
		CoverageSummary: CoverageSummary{GateStatus: "WARNING"}}

	must(t, WriteTraceability(specsDir, slug, m))
	must(t, WriteTraceability(specsDir, slug, m)) // second call

	got, err := ReadTraceability(specsDir, slug)
	if err != nil {
		t.Fatalf("ReadTraceability after second write: %v", err)
	}
	if got.Feature != slug {
		t.Errorf("Feature corrupted after idempotent write: got %q", got.Feature)
	}
}

func TestWriteTraceability_PathTraversal_Rejected(t *testing.T) {
	// D6 AuthZ: slug "../escape" must be rejected before any write
	specsDir := t.TempDir()
	m := TraceabilityMatrix{Feature: "../escape", Generated: "2026-06-02T00:00:00Z"}

	err := WriteTraceability(specsDir, "../escape", m)

	if err == nil {
		t.Error("WriteTraceability must reject path-traversal slug ../escape")
	}
}

func TestWriteTraceability_EmptyMatrix_ZeroCoverage(t *testing.T) {
	// D2 boundary: zero ACs
	specsDir := t.TempDir()
	slug := "zero"
	must(t, os.MkdirAll(filepath.Join(specsDir, slug), 0o755))

	m := TraceabilityMatrix{Feature: slug, Generated: "2026-06-02T00:00:00Z", Matrix: nil,
		CoverageSummary: CoverageSummary{TotalACs: 0, GateStatus: "WARNING"}}
	must(t, WriteTraceability(specsDir, slug, m))

	got, err := ReadTraceability(specsDir, slug)
	if err != nil {
		t.Fatalf("ReadTraceability: %v", err)
	}
	if got.CoverageSummary.TotalACs != 0 {
		t.Errorf("TotalACs: want 0, got %d", got.CoverageSummary.TotalACs)
	}
}

// ── AC-005 / AC-012: gate_status + CI Alignment ───────────────────────────────

func TestCheckTestFilesExist_AllPresent_ReturnsAllPresent(t *testing.T) {
	root := t.TempDir()
	f1 := filepath.Join(root, "foo_test.go")
	f2 := filepath.Join(root, "bar_test.go")
	must(t, os.WriteFile(f1, []byte("package x\n"), 0o600))
	must(t, os.WriteFile(f2, []byte("package x\n"), 0o600))

	result := CheckTestFilesExist([]string{f1, f2})

	if !result.AllPresent {
		t.Errorf("all files present but AllPresent=false; missing: %v", result.Missing)
	}
}

func TestCheckTestFilesExist_SomeMissing_ReturnsNotAllPresent(t *testing.T) {
	root := t.TempDir()
	f1 := filepath.Join(root, "exists_test.go")
	must(t, os.WriteFile(f1, []byte("package x\n"), 0o600))
	f2 := filepath.Join(root, "missing_test.go") // not written

	result := CheckTestFilesExist([]string{f1, f2})

	if result.AllPresent {
		t.Error("one file missing but AllPresent=true")
	}
	if len(result.Missing) != 1 || result.Missing[0] != f2 {
		t.Errorf("Missing should contain f2, got %v", result.Missing)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// testRoot creates a temp dir with the appropriate stack marker file.
func testRoot(t *testing.T, stack string) (root, slug string) {
	t.Helper()
	root = t.TempDir()
	slug = "testslug"
	must(t, os.MkdirAll(filepath.Join(root, ".forge", "specs", slug), 0o755))
	switch stack {
	case "go":
		must(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\ngo 1.22\n"), 0o600))
	case "ts":
		must(t, os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"devDependencies":{"jest":"^29"}}`), 0o600))
	case "py":
		must(t, os.WriteFile(filepath.Join(root, "requirements.txt"), []byte("pytest==8.0.0\n"), 0o600))
	}
	return root, slug
}

// allScores returns a slice of 9 DimensionScore with all scores set to v and Covered=true.
func allScores(v float64) []DimensionScore {
	scores := make([]DimensionScore, 9)
	for i := range 9 {
		scores[i] = DimensionScore{
			Dim:     Dimension(i + 1),
			Score:   v,
			Covered: true,
		}
	}
	return scores
}
