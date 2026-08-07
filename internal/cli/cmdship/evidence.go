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

// evidence.go — M1: a checkpoint may not report "ok" on forge's own say-so.
//
// # The failure this closes
//
// M2 asked whether the gates check anything. M3 gave them a way to say "I
// could not check". This asks the question one level up: **what is a green
// checkpoint standing on?**
//
// `Checkpoint.Status` is a plain string that any of twenty-odd code paths can
// set to "ok". Nothing has ever required the code setting it to say why. And
// because forge is the actor in every one of those paths, the thing it is
// implicitly asserting is always true — "I wrote the file", "I ran the
// generator", "I completed the step". Those are facts about forge's own
// behaviour, not about whether the change is sound. Every bug in this family
// has had the same shape:
//
//	spec.md written        ≠  spec.md is complete
//	test file written      ≠  the test will ever run
//	the gate returned      ≠  the gate examined anything
//	the checkpoint ran     ≠  the checkpoint verified anything
//
// # What counts as evidence
//
// Evidence is an observation about the world that did not come from forge
// asserting its own success. Two sources qualify:
//
//	SourceExternalTool — something outside forge was asked and answered:
//	                     a test runner, a scanner, git, a linter.
//	SourceReadBack     — forge re-read what landed on disk and re-validated
//	                     it, judging the artefact as it would judge a
//	                     stranger's, rather than trusting the value it just
//	                     held in memory.
//
// And one does not:
//
//	SourceForgeClaim   — "I did the thing." Recorded, reported, and never
//	                     sufficient on its own to make a checkpoint green.
//
// # How it is enforced
//
// Not by making Status private or routing every assignment through a setter —
// that invites a SourceForgeClaim boilerplate that satisfies the compiler and
// nothing else. It is enforced where it matters, at the reporting boundary: a
// checkpoint that reaches "ok" with no qualifying evidence is downgraded to
// "warning" and annotated UNVERIFIED.
//
// The downgrade is deliberately not a failure. The claim being made is "nobody
// checked", which is a reason to withhold confidence, not a reason to block a
// release. `res.Ready` is unaffected — it turns on "fail", not "warning" — so
// this cannot break a pipeline that was working.
package cmdship

import (
	"fmt"
	"strings"
)

// EvidenceSource says where an observation came from, which is the only thing
// that makes it worth anything.
type EvidenceSource string

const (
	// SourceExternalTool — a tool outside forge was asked and answered.
	SourceExternalTool EvidenceSource = "external-tool"
	// SourceReadBack — forge re-read the artefact from disk and re-validated it.
	SourceReadBack EvidenceSource = "read-back"
	// SourceForgeClaim — forge asserting its own success. Never sufficient.
	SourceForgeClaim EvidenceSource = "forge-claim"
)

// Independent reports whether this source counts toward a green checkpoint.
//
// The name is the point: evidence is only worth having when it is independent
// of the party being assessed, and forge assessing forge is not.
func (s EvidenceSource) Independent() bool {
	return s == SourceExternalTool || s == SourceReadBack
}

// Evidence is one observation supporting a checkpoint's status.
type Evidence struct {
	// Source is where the observation came from.
	Source EvidenceSource `json:"source"`
	// Claim is what it establishes, in the checkpoint's own terms
	// (e.g. "spec.md has an Acceptance Criteria section with Given/When/Then").
	Claim string `json:"claim"`
	// Observed is the raw finding, kept short — the count, the exit code, the
	// gate name. It is what a reviewer would ask for when the claim is
	// disputed.
	Observed string `json:"observed,omitempty"`
}

// String renders one evidence entry for a checkpoint detail line.
func (e Evidence) String() string {
	if e.Observed == "" {
		return string(e.Source) + ": " + e.Claim
	}
	return string(e.Source) + ": " + e.Claim + " (" + e.Observed + ")"
}

// AddEvidence records an observation on the checkpoint.
func (cp *Checkpoint) AddEvidence(source EvidenceSource, claim, observed string) {
	if cp == nil {
		return
	}
	cp.Evidence = append(cp.Evidence, Evidence{Source: source, Claim: claim, Observed: observed})
}

// HasIndependentEvidence reports whether anything other than forge's own
// say-so supports this checkpoint.
func (cp *Checkpoint) HasIndependentEvidence() bool {
	if cp == nil {
		return false
	}
	for _, e := range cp.Evidence {
		if e.Source.Independent() {
			return true
		}
	}
	return false
}

// EvidenceSummary renders the independent evidence for a detail line, or "" if
// there is none.
func (cp *Checkpoint) EvidenceSummary() string {
	if cp == nil {
		return ""
	}
	var parts []string
	for _, e := range cp.Evidence {
		if e.Source.Independent() {
			parts = append(parts, e.String())
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "; ")
}

// applyEvidencePolicy is the enforcement point.
//
// A checkpoint that reached "ok" without a single independent observation is
// reporting confidence it has not earned, so it is downgraded to "warning" and
// told to say so. Statuses other than "ok" are left alone: a checkpoint that
// already failed has a real finding to report, and one already at "warning" is
// not claiming anything that needs qualifying.
func applyEvidencePolicy(cp *Checkpoint) {
	if cp == nil || cp.Status != "ok" {
		return
	}
	if cp.HasIndependentEvidence() {
		return
	}
	cp.Status = "warning"
	cp.Detail += fmt.Sprintf(
		" | UNVERIFIED[%s reported ok with no independent evidence: nothing outside forge "+
			"confirmed this, and no artefact was re-read and re-validated]",
		strings.ToLower(cp.Name))
}
