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

// Test-design checklist (always-write-tests.md 9-point):
//  1. Happy path           — a checkpoint with real evidence stays green.
//  2. Boundary             — evidence present but all of it forge's own claim.
//  3. Negative             — green with no evidence at all is downgraded.
//  4. Idempotency          — applying the policy twice changes nothing further.
//  5. Concurrency          — every case owns its TempDir.
//  6. Cross-cutting        — a real pipeline run records evidence, so the
//     policy is neither vacuously satisfied nor
//     vacuously triggered.
//  7. Regression           — the policy must never turn a working run red.
//  8. Data-accuracy        — evidence renders with its source and claim.
//  9. False-positive guard — fail/warning/skipped checkpoints are untouched.
package cmdship

import (
	"strings"
	"testing"
)

// ── Negative: the whole point ─────────────────────────────────────────────────

func TestEvidencePolicy_DowngradesGreenWithNoEvidence(t *testing.T) {
	t.Parallel()
	cp := Checkpoint{Name: "Spec", Status: "ok", Detail: "spec written"}
	applyEvidencePolicy(&cp)

	if cp.Status != "warning" {
		t.Fatalf("a green checkpoint resting on nothing but forge's own say-so must not "+
			"stay green, got %q", cp.Status)
	}
	if !strings.Contains(cp.Detail, "UNVERIFIED") {
		t.Fatalf("the downgrade must be visible in the detail: %q", cp.Detail)
	}
	if !strings.Contains(cp.Detail, "no independent evidence") {
		t.Fatalf("the detail must say why, not just that something is off: %q", cp.Detail)
	}
}

// ── Boundary: forge's own claim is not evidence ───────────────────────────────

func TestEvidencePolicy_ForgeClaimAloneIsNotEnough(t *testing.T) {
	t.Parallel()
	cp := Checkpoint{Name: "Spec", Status: "ok"}
	// "I wrote the file" is a fact about forge's behaviour, not about whether
	// the change is sound. If this ever counted, the type would be decoration.
	cp.AddEvidence(SourceForgeClaim, "spec.md was written", "1 file")
	applyEvidencePolicy(&cp)

	if cp.Status != "warning" {
		t.Fatal("SourceForgeClaim must never be sufficient on its own — otherwise every " +
			"checkpoint can self-certify and the policy means nothing")
	}
}

// ── Happy path ────────────────────────────────────────────────────────────────

func TestEvidencePolicy_IndependentEvidenceKeepsItGreen(t *testing.T) {
	t.Parallel()
	for _, src := range []EvidenceSource{SourceExternalTool, SourceReadBack} {
		cp := Checkpoint{Name: "Ship", Status: "ok", Detail: "clean"}
		cp.AddEvidence(src, "scanner reported no findings", "0 findings")
		applyEvidencePolicy(&cp)
		if cp.Status != "ok" {
			t.Errorf("%s is independent evidence and must keep the checkpoint green, got %q",
				src, cp.Status)
		}
	}
}

// ── False-positive guard ──────────────────────────────────────────────────────

func TestEvidencePolicy_LeavesNonGreenStatusesAlone(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"fail", "warning", "skipped"} {
		cp := Checkpoint{Name: "Spec", Status: status, Detail: "original"}
		applyEvidencePolicy(&cp)
		if cp.Status != status {
			t.Errorf("status %q must not be rewritten; got %q", status, cp.Status)
		}
		if cp.Detail != "original" {
			t.Errorf("status %q must not be annotated; got %q", status, cp.Detail)
		}
	}
}

// ── Idempotency ───────────────────────────────────────────────────────────────

func TestEvidencePolicy_IsIdempotent(t *testing.T) {
	t.Parallel()
	cp := Checkpoint{Name: "Spec", Status: "ok", Detail: "d"}
	applyEvidencePolicy(&cp)
	first := cp.Detail
	applyEvidencePolicy(&cp)
	if cp.Detail != first {
		t.Fatalf("re-applying must not stack annotations:\n first: %q\nsecond: %q", first, cp.Detail)
	}
}

// ── Source classification ─────────────────────────────────────────────────────

func TestEvidenceSource_IndependenceIsExplicit(t *testing.T) {
	t.Parallel()
	if !SourceExternalTool.Independent() || !SourceReadBack.Independent() {
		t.Error("asking a tool, and re-reading an artefact from disk, are both independent")
	}
	if SourceForgeClaim.Independent() {
		t.Fatal("forge assessing forge is not independent evidence; if this ever returns " +
			"true the entire policy silently stops doing anything")
	}
	// An unrecognised source must not sneak through as independent: a new
	// source has to be argued for, not inherited by accident.
	if EvidenceSource("something-new").Independent() {
		t.Fatal("an unknown evidence source must default to NOT independent")
	}
}

// ── Cross-cutting: the policy is not vacuous in a real run ────────────────────

// TestEvidencePolicy_RealRunRecordsEvidence guards both ways this can go wrong
// at once. If the pipeline recorded no evidence anywhere, the policy would
// downgrade every run and become noise people learn to ignore. If it recorded
// evidence unconditionally, the policy would never fire and the type would be
// decoration.
func TestEvidencePolicy_RealRunRecordsEvidence(t *testing.T) {
	t.Parallel()
	res := RunWithOptions(RunOptions{
		Root:            t.TempDir(),
		Description:     "evidence policy feature",
		NoStrictTesting: true,
	})

	var withEvidence, greenWithout int
	for _, cp := range res.Checkpoints {
		if cp.HasIndependentEvidence() {
			withEvidence++
		}
		if cp.Status == "ok" && !cp.HasIndependentEvidence() {
			greenWithout++
		}
	}
	if withEvidence == 0 {
		t.Fatal("no checkpoint in a full run recorded independent evidence — the policy " +
			"would downgrade every run, which is how a safety check gets switched off")
	}
	if greenWithout > 0 {
		t.Fatalf("%d checkpoint(s) are green with no independent evidence; "+
			"applyEvidencePolicy is not reaching them", greenWithout)
	}
}

// ── Regression: never turn a working pipeline red ─────────────────────────────

func TestEvidencePolicy_NeverBlocksARun(t *testing.T) {
	t.Parallel()
	res := RunWithOptions(RunOptions{
		Root:            t.TempDir(),
		Description:     "evidence policy feature",
		NoStrictTesting: true,
	})
	// The claim a downgrade makes is "nobody checked" — a reason to withhold
	// confidence, not to block a release. res.Ready keys on "fail", and this
	// policy must never produce one.
	for _, cp := range res.Checkpoints {
		if cp.Status == "fail" && strings.Contains(cp.Detail, "no independent evidence") {
			t.Fatalf("the evidence policy escalated a checkpoint to fail: %s", cp.Detail)
		}
	}
}

// ── Data accuracy ─────────────────────────────────────────────────────────────

func TestEvidence_String_CarriesSourceAndClaim(t *testing.T) {
	t.Parallel()
	e := Evidence{Source: SourceExternalTool, Claim: "scan clean", Observed: "0 findings"}
	got := e.String()
	for _, want := range []string{"external-tool", "scan clean", "0 findings"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered evidence missing %q: %s", want, got)
		}
	}
	// Observed is optional and must not leave stray punctuation behind.
	bare := Evidence{Source: SourceReadBack, Claim: "spec.md has ACs"}.String()
	if strings.Contains(bare, "()") {
		t.Errorf("empty Observed must be omitted cleanly: %s", bare)
	}
}
