# Arch: Test Phase Quality Framework (RFC-005 §6)

**Type**: T1 Micro — 3 roles, 1 round  
**Feature**: test-phase-quality-framework  
**Date**: 2026-06-02  

---

## Component Topology

```
internal/cli/cmdship/
  workspace_context.go      EXTEND  — add TestFrameworkContext struct + detectTestFramework()
  test_artifacts.go         EXTEND  — add writeTestArtifactsWithContext(); add bugFixSignals
  test_scoring.go           CREATE  — 9-dimension rubric, ComputeCompositeScore(), gate logic
  traceability.go           CREATE  — TraceabilityMatrix, WriteTraceability(), ReadTraceability()
  test_review.go            CREATE  — 3-role parallel debate + synthesis
```

---

## Role Assignments (T1 Micro)

**Role A — QA Architect**  
Mandate: AC-001..AC-006 (P1). TestFrameworkContext detection, framework-aware stub
generation, bug-fix regression guard, dimension warning emission.

**Role B — Security Tester**  
Mandate: AC-008..AC-009 (P2 scoring). Block gate when thresholds breached; T0/T2 tier
waivers. Ensure no path traversal via slugs; validate `root` via fssandbox.

**Role C — Reliability Tester**  
Mandate: AC-010..AC-014 (P2 traceability, CI gate). WriteTraceability, ReadTraceability,
CI Alignment Check, run_tests_on_verify, test_debate_threshold config.

---

## API Contract

### `workspace_context.go` additions

```go
// TestFrameworkContext holds the detected test environment for the workspace.
// All fields are populated by deterministic filesystem inspection (zero LLM calls).
// RFC-005 §6.3
type TestFrameworkContext struct {
    Language       string   // "go" | "python" | "typescript" | "java" | ""
    TestRunner     string   // "go test" | "pytest" | "jest" | "junit" | ""
    AssertionStyle string   // "testify" | "assert" | "chai" | "hamcrest" | ""
    MockLibrary    string   // "gomock" | "unittest.mock" | "jest.mock" | "mockito" | ""
    CoverageCmd    string   // "go test -cover ./..." | "pytest --cov" | "jest --coverage" | ""
    FuzzSupport    bool     // true if go 1.18+ or hypothesis/atheris detected
    IntegTestDir   string   // "tests/integration/" | "test/int/" | "" if absent
    FixtureDir     string   // "tests/fixtures/" | "testdata/" | "" if absent
    ExistingTests  []string // sample of up to 5 existing test file names
}

// detectTestFramework scans root deterministically and returns TestFrameworkContext.
// Never makes LLM calls. Safe to call from hot paths.
func detectTestFramework(root string) TestFrameworkContext
```

`WorkspaceContextResult` gets a new field: `TestFW TestFrameworkContext`.

### `test_artifacts.go` additions

```go
// bugFixSignals is the set of keywords that trigger D7 mandatory enforcement.
var bugFixSignals = []string{
    "fix", "bug", "issue", "regression", "broken", "crash",
    "nil pointer", "panic", "incorrect", "wrong output",
    "not working", "doesn't work", "race condition",
}

// IsBugFix returns true when feature description contains any bugFixSignal.
func IsBugFix(feature string) bool

// TestArtifactPaths extended fields (additive — existing fields kept for compat):
//   GoTest        string  // <slug>_test.go
//   GoFuzzTest    string  // <slug>_fuzz_test.go (only when FuzzSupport=true)
//   PyTest        string  // test_<slug>.py
//   JavaTest      string  // <Slug>Test.java
//   Traceability  string  // .forge/specs/<slug>/traceability.yaml

// writeTestArtifactsWithContext is the framework-aware replacement.
// writeTestArtifacts remains as a shim that calls this with detectTestFramework(root).
func writeTestArtifactsWithContext(
    root, slug, feature, specMD string,
    fw TestFrameworkContext,
    pipe *LLMPipe,
) TestArtifactPaths
```

### `test_scoring.go` (NEW)

```go
// Dimension identifies one of the 9 test quality dimensions (D1–D9).
type Dimension int

const (
    D1HappyPath      Dimension = iota + 1 // weight 1.0, threshold 7
    D2Boundary                             // weight 1.5, threshold 6
    D3Negative                             // weight 1.5, threshold 6
    D4Idempotency                          // weight 1.2, threshold 5
    D5Concurrency                          // weight 1.2, threshold 5
    D6AuthZ                                // weight 2.0, threshold 8
    D7Regression                           // weight 1.0, threshold 7
    D8DataAccuracy                         // weight 1.3, threshold 6
    D9FalsePositive                        // weight 1.2, threshold 6
)

type DimensionScore struct {
    Dim       Dimension
    Score     float64 // 0–10
    Covered   bool    // true if at least one test targets this dimension
}

type ScoreResult struct {
    Scores           []DimensionScore
    CompositeScore   float64
    GateStatus       string   // "PASS" | "WARNING" | "BLOCK"
    MissingDims      []Dimension
    WaivedDims       []Dimension
    BlockingReasons  []string // human-readable reasons for BLOCK
}

// ComputeCompositeScore computes the weighted composite score.
// tier: "T0" | "T1" | "T2". isBugFix forces D7 mandatory ≥8.
func ComputeCompositeScore(scores []DimensionScore, tier string, isBugFix bool, waivers []Dimension) ScoreResult
```

### `traceability.go` (NEW)

```go
// TraceabilityMatrix is the machine-readable .forge/specs/<slug>/traceability.yaml
// per RFC-005 §6.5.
type TraceabilityMatrix struct {
    Feature         string      `yaml:"feature"`
    Generated       string      `yaml:"generated"` // RFC3339
    SpecVersion     string      `yaml:"spec_version"` // sha256:<hash>
    Matrix          []ACEntry   `yaml:"matrix"`
    CoverageSummary CoverageSummary `yaml:"coverage_summary"`
}

type ACEntry struct {
    ACID   string      `yaml:"ac_id"`
    ACText string      `yaml:"ac_text"`
    Tests  []TestEntry `yaml:"tests"`
}

type TestEntry struct {
    ID        string `yaml:"id"`
    Name      string `yaml:"name"`
    Type      string `yaml:"type"`      // unit|integration|performance|fuzz
    Dimension string `yaml:"dimension"` // D1..D9
    File      string `yaml:"file"`
}

type CoverageSummary struct {
    TotalACs          int              `yaml:"total_acs"`
    ACsWithTests      int              `yaml:"acs_with_tests"`
    TotalTests        int              `yaml:"total_tests"`
    DimensionCoverage map[string]bool  `yaml:"dimension_coverage"` // "D1"..
    CompositeScore    float64          `yaml:"composite_score"`
    MissingDimensions []string         `yaml:"missing_dimensions"`
    WaivedDimensions  []string         `yaml:"waived_dimensions"`
    GateStatus        string           `yaml:"gate_status"` // PASS|WARNING|BLOCK
}

func WriteTraceability(specsDir, slug string, m TraceabilityMatrix) error
func ReadTraceability(specsDir, slug string) (TraceabilityMatrix, error)
```

### `test_review.go` (NEW)

```go
// TestReviewResult holds the synthesized output of the 3-role parallel debate.
type TestReviewResult struct {
    QAFindings         string
    SecurityFindings   string
    ReliabilityFindings string
    Synthesis          string
    DimensionGaps      []Dimension // dimensions flagged as missing by any role
}

// RunTestDebate executes the 3-role parallel review.
// Returns immediately with a single-role result when tier == "T0" and
// ship.test_debate_threshold config is "T0".
func RunTestDebate(ctx context.Context, slug, feature, testPlan string, fw TestFrameworkContext, pipe *LLMPipe) TestReviewResult
```

---

## Traceability.yaml Initial State (written at Test checkpoint)

```yaml
feature: <slug>
generated: <RFC3339>
spec_version: "sha256:<hash-of-spec.md>"
matrix:
  - ac_id: AC-001
    ac_text: "<from spec.yml>"
    tests: []   # populated at Code checkpoint
coverage_summary:
  total_acs: 14
  acs_with_tests: 0
  total_tests: 0
  dimension_coverage:
    D1: false
    D2: false
    D3: false
    D4: false
    D5: false
    D6: false
    D7: false
    D8: false
    D9: false
  composite_score: 0.0
  missing_dimensions: [D1, D2, D3, D4, D5, D6, D7, D8, D9]
  waived_dimensions: []
  gate_status: WARNING   # initial: no tests yet; block escalates after code checkpoint
```

---

## Security Threat Model

| Threat | Mitigation |
|---|---|
| Path traversal via slug | `fssandbox.ValidateRoot(filepath.Join(root, ".forge", "specs", slug))` before any write |
| Malformed spec.yml injection | yaml.v3 strict decode with known-field validation; reject unknown keys |
| Coverage command injection (AC-013) | `procspawn` allow-list; `CoverageCmd` validated against known patterns |
| Traceability file size bomb | Cap matrix at 500 AC entries; cap test entries per AC at 50 |

---

## ADR References

| ADR | Relevance |
|---|---|
| ADR-001 | Go 1.24+, CGO_ENABLED=0 — no C libs in fuzz detection |
| ADR-009 | Error codes: new codes in FORGE-3210..3219 range |
| ADR-014 | Resilience: WriteTraceability uses retry on transient fs errors |
| ADR-024 | Reversibility: WriteTraceability is idempotent (overwrite-safe) |
