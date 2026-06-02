# Breakdown: Test Phase Quality Framework (RFC-005 §6)

**Slug**: test-phase-quality-framework  
**Phase scope**: P1 (T-001..T-010) → P2 (T-011..T-020)  

---

## P1 Tasks

### T-001 — Add `TestFrameworkContext` struct to `workspace_context.go`
File: `internal/cli/cmdship/workspace_context.go`  
Change: Add `TestFrameworkContext` struct with all 9 fields per RFC §6.3:
`Language`, `TestRunner`, `AssertionStyle`, `MockLibrary`, `CoverageCmd`,
`FuzzSupport`, `IntegTestDir`, `FixtureDir`, `ExistingTests []string`.
Add `TestFW TestFrameworkContext` field to `WorkspaceContextResult`.  
Effort: XS  
Done when: AC-001  
Depends on: none  

### T-002 — Implement `detectTestFramework(root string)` in `workspace_context.go`
File: `internal/cli/cmdship/workspace_context.go`  
Change: Deterministic zero-LLM scan: go.mod (parse go version for fuzz boundary 1.18),
package.json (detect jest), requirements.txt / pyproject.toml (detect pytest + hypothesis),
pom.xml / build.gradle (detect junit). Detect IntegTestDir (tests/integration/, test/int/).
Detect FixtureDir (tests/fixtures/, testdata/). Collect up to 5 ExistingTests file names.  
Effort: S  
Done when: AC-002  
Depends on: T-001  

### T-003 — Wire `detectTestFramework` into `collectWorkspaceContext`
File: `internal/cli/cmdship/workspace_context.go`  
Change: Call `detectTestFramework(root)` inside `collectWorkspaceContext()`; populate
`res.TestFW`; append a `## Test Framework` section to the markdown snapshot (≤100 chars).  
Effort: XS  
Done when: AC-002, AC-003  
Depends on: T-002  

### T-004 — Extend `TestArtifactPaths` with framework-specific fields
File: `internal/cli/cmdship/test_artifacts.go`  
Change: Add fields `GoTest`, `GoFuzzTest`, `PyTest`, `JavaTest`, `Traceability string`
to `TestArtifactPaths`. Existing fields (UnitTest, IntegrationTest, RLSTest, ScanBaseline)
kept for backward compatibility.  
Effort: XS  
Done when: AC-006  
Depends on: T-001  

### T-005 — Add `bugFixSignals` and `IsBugFix()` to `test_artifacts.go`
File: `internal/cli/cmdship/test_artifacts.go`  
Change: Add `var bugFixSignals = []string{...}` per RFC §6.6. Add exported `IsBugFix(feature string) bool`.  
Effort: XS  
Done when: AC-004  
Depends on: none  

### T-006 — Add Go test stub generators to `test_artifacts.go`
File: `internal/cli/cmdship/test_artifacts.go`  
Change: Add `goTestStub(slug, feature string, isBugFix bool) string` — returns a
`package cmdship` test file with `func TestXxx_HappyPath`, `func TestXxx_ErrorPath`,
and when isBugFix: two `// Regression:` stubs. Add `goFuzzTestStub(slug string) string`.  
Effort: S  
Done when: AC-006, AC-004  
Depends on: T-005  

### T-007 — Add Python and Java stub generators to `test_artifacts.go`
File: `internal/cli/cmdship/test_artifacts.go`  
Change: Add `pyTestStub(slug, feature string) string` (pytest class format).
Add `javaTestStub(slug, feature string) string` (JUnit 5 format).  
Effort: S  
Done when: AC-006  
Depends on: none  

### T-008 — Implement `writeTestArtifactsWithContext` in `test_artifacts.go`
File: `internal/cli/cmdship/test_artifacts.go`  
Change: Add `writeTestArtifactsWithContext(root, slug, feature, specMD string, fw TestFrameworkContext, pipe *LLMPipe) TestArtifactPaths`.
Dispatch to language-specific generators: go → goTestStub + goFuzzTestStub (when FuzzSupport),
python → pyTestStub, typescript → existing unitTestStub/integrationTestStub, java → javaTestStub.
RLS stub only when `fw.Language == "typescript"` (Supabase pattern — RFC §6.3).
Update `writeTestArtifacts` to be a shim calling `writeTestArtifactsWithContext` with `detectTestFramework(root)`.  
Effort: M  
Done when: AC-003, AC-006  
Depends on: T-004, T-005, T-006, T-007  

### T-009 — Add `DimensionCoverageResult` and `gate_status WARNING` emission (P1 scope)
File: `internal/cli/cmdship/test_artifacts.go` (or new `test_gate.go`)  
Change: After writing stubs, compute which of D1–D9 are covered based on stub content
(heuristic: D1=any test present, D6=RLS/auth test present, D7=Regression label present).
Return `GateStatus: "WARNING"` in a new `TestCheckpointResult` when any dim is uncovered.
Do NOT block the pipeline (P1 — emit warning only).  
Effort: S  
Done when: AC-005  
Depends on: T-008  

### T-010 — Add error codes for test framework errors
File: `internal/cli/cmdship/ship.go` (error registrations)  
Change: Register `ErrFrameworkDetectFailed = errcode.Register(errcode.Code(3210), "...")`,
`ErrTraceabilityWriteFailed = errcode.Register(errcode.Code(3211), "...")`,
`ErrTestGateBLOCK = errcode.Register(errcode.Code(3212), "...")`.  
Effort: XS  
Done when: AC-001  
Depends on: none  

---

## P2 Tasks

### T-011 — Create `test_scoring.go` — dimension types + weights
File: `internal/cli/cmdship/test_scoring.go` (CREATE)  
Change: Define `Dimension int` type, `D1HappyPath..D9FalsePositive` constants,
`dimensionMeta` map with weight and threshold per §6.2 table.
Define `DimensionScore`, `ScoreResult` structs.  
Effort: S  
Done when: AC-007  
Depends on: none  

### T-012 — Implement `ComputeCompositeScore` in `test_scoring.go`
File: `internal/cli/cmdship/test_scoring.go`  
Change: Implement `ComputeCompositeScore(scores []DimensionScore, tier string, isBugFix bool, waivers []Dimension) ScoreResult`.
Apply weighted average formula. Apply waiver logic (T0: skip D5/D6/D8; T2: D6 threshold→9).
Apply bug-fix D7 override (threshold→8). Set GateStatus: BLOCK when composite<6.5 or
high-weight dim below threshold; WARNING when dims missing; PASS otherwise.  
Effort: M  
Done when: AC-007, AC-008, AC-009  
Depends on: T-011  

### T-013 — Create `traceability.go` — types + file I/O
File: `internal/cli/cmdship/traceability.go` (CREATE)  
Change: Define `TraceabilityMatrix`, `ACEntry`, `TestEntry`, `CoverageSummary` structs
per RFC §6.5. Implement `WriteTraceability(specsDir, slug string, m TraceabilityMatrix) error`
with fssandbox path validation and yaml.v3 marshal. Implement `ReadTraceability(specsDir, slug string) (TraceabilityMatrix, error)`.  
Effort: M  
Done when: AC-010  
Depends on: T-010  

### T-014 — Write initial traceability.yaml at Test checkpoint
File: `internal/cli/cmdship/test_artifacts.go`  
Change: After `writeTestArtifactsWithContext`, call `WriteTraceability` with all ACs from
spec.yml in `missing` state (matrix populated from spec, tests[]=[], gate_status=WARNING).
Store path in `paths.Traceability`.  
Effort: S  
Done when: AC-010  
Depends on: T-008, T-013  

### T-015 — Create `test_review.go` — 3-role parallel debate
File: `internal/cli/cmdship/test_review.go` (CREATE)  
Change: Implement `RunTestDebate(ctx, slug, feature, testPlan string, fw TestFrameworkContext, pipe *LLMPipe) TestReviewResult`.
Three goroutines issue concurrent LLM calls (QA Architect, Security Tester, Reliability Tester).
Collect via channel. Synthesis call with QA Architect persona. Handle `pipe == nil` gracefully (dry-run).  
Effort: M  
Done when: AC-011  
Depends on: T-001  

### T-016 — Implement `ship.test_debate_threshold` config gate in `test_review.go`
File: `internal/cli/cmdship/test_review.go`  
Change: Read `ship.test_debate_threshold` from config before launching debate.
When value is `"T0"` and feature tier is T0: skip 3-role debate, run single QA Architect pass only.  
Effort: XS  
Done when: AC-014  
Depends on: T-015  

### T-017 — Create `CheckTestFilesExist` function (CI Alignment Check)
File: `internal/cli/cmdship/test_artifacts.go` (or `test_gate.go`)  
Change: Implement `CheckTestFilesExist(files []string) CIAlignmentResult` where
`CIAlignmentResult` has `AllPresent bool`, `Present []string`, `Missing []string`,
`CoverageSnippet string`. Emit snippet to stdout via the calling checkpoint.  
Effort: S  
Done when: AC-012  
Depends on: T-013  

### T-018 — Implement `ship.run_tests_on_verify` in QA-Verify checkpoint
File: `internal/cli/cmdship/ship.go` (or `verify.go`)  
Change: In `checkVerify`, read `ship.run_tests_on_verify` config. When true: execute
`fw.CoverageCmd` via `procspawn`; non-zero exit → add gap to `SpecAuditResult.Gaps` →
remediation loop. Graceful skip when env is incomplete (procspawn returns err).  
Effort: S  
Done when: AC-013  
Depends on: T-013, T-017  

### T-019 — Wire `ScoreResult` into test checkpoint output
File: `internal/cli/cmdship/ship.go` (or `test_artifacts.go`)  
Change: After `writeTestArtifactsWithContext`, call `ComputeCompositeScore` with initial
heuristic scores. Embed `ScoreResult.GateStatus` in `TestCheckpointResult`. Halt pipeline
with `ErrTestGateBLOCK` when `GateStatus == "BLOCK"`.  
Effort: S  
Done when: AC-008  
Depends on: T-012, T-008  

### T-020 — Update `traceability.yaml` `spec_version` from spec.md sha256
File: `internal/cli/cmdship/traceability.go`  
Change: In `WriteTraceability`, compute `sha256:<hex>` of spec.md content; write as
`spec_version` field. Update `ReadTraceability` to expose the hash.  
Effort: XS  
Done when: AC-010  
Depends on: T-013  
