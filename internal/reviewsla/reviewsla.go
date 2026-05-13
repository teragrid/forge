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

// Package reviewsla implements M3-08: maintainer review-SLA tracking.
//
// SLA targets (per CONTRIBUTING.md and docs/MAINTAINER_SLA.md):
//
//	Initial triage   — 48 hours after issue/PR creation
//	First review     — 7 calendar days after PR submission
//	Merge decision   — 30 calendar days after PR submission
//
// The package reads a JSONL snapshot file produced by `forge sla snapshot`
// (which queries the GitHub API and stores raw issue/PR metadata offline).
// The Checker can also be fed records directly, enabling tests without
// network access.
package reviewsla

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/teragrid/forge/internal/errcode"
)

// Error codes (range 5850..5899).
var (
	ErrSLACheck = errcode.Register(errcode.Code(5850), "review-SLA check failed")
)

// SLAPolicy holds the configurable SLA targets.
type SLAPolicy struct {
	// TriageWithin is the time window for initial triage after creation.
	TriageWithin time.Duration
	// FirstReviewWithin is the target window for a first substantive review.
	FirstReviewWithin time.Duration
	// MergeDecisionWithin is the target window for a merge/reject decision.
	MergeDecisionWithin time.Duration
}

// DefaultPolicy is the canonical SLA for the forge project.
var DefaultPolicy = SLAPolicy{
	TriageWithin:        48 * time.Hour,
	FirstReviewWithin:   7 * 24 * time.Hour,
	MergeDecisionWithin: 30 * 24 * time.Hour,
}

// PRRecord is the minimal metadata for a pull request.
type PRRecord struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	// FirstReviewAt is zero if no review has been posted yet.
	FirstReviewAt time.Time `json:"first_review_at,omitempty"`
	// MergedAt / ClosedAt are zero if the PR is still open.
	MergedAt time.Time `json:"merged_at,omitempty"`
	ClosedAt time.Time `json:"closed_at,omitempty"`
	Labels   []string  `json:"labels,omitempty"`
	State    string    `json:"state"` // "open" | "closed" | "merged"
}

// SLAResult summarises SLA compliance for a single PR.
type SLAResult struct {
	PR                    PRRecord
	TriageBreached        bool
	FirstReviewBreached   bool
	MergeDecisionBreached bool
	// Age is the time elapsed since PR creation (as of the evaluation time).
	Age time.Duration
}

// String returns a one-line human-readable summary.
func (r SLAResult) String() string {
	breach := ""
	if r.TriageBreached {
		breach += " [TRIAGE]"
	}
	if r.FirstReviewBreached {
		breach += " [REVIEW]"
	}
	if r.MergeDecisionBreached {
		breach += " [MERGE]"
	}
	if breach == "" {
		breach = " [ok]"
	}
	return fmt.Sprintf("PR#%d %q age=%s%s", r.PR.Number, r.PR.Title, r.Age.Round(time.Hour), breach)
}

// Checker evaluates PRs against an SLA policy.
type Checker struct {
	policy SLAPolicy
	now    func() time.Time // injectable for tests
}

// NewChecker returns a Checker using the given policy and real clock.
func NewChecker(p SLAPolicy) *Checker {
	return &Checker{policy: p, now: time.Now}
}

// Check evaluates a single PRRecord against the SLA policy.
func (c *Checker) Check(pr PRRecord) SLAResult {
	now := c.now()
	age := now.Sub(pr.CreatedAt)

	// Triage SLA: always applies to open PRs within triage window.
	triageBreached := pr.State == "open" && age > c.policy.TriageWithin

	// First-review SLA: breached when no review has been posted.
	firstReviewBreached := pr.State == "open" &&
		pr.FirstReviewAt.IsZero() &&
		age > c.policy.FirstReviewWithin

	// Merge-decision SLA: breached when still open past decision window.
	mergeBreached := pr.State == "open" && age > c.policy.MergeDecisionWithin

	return SLAResult{
		PR:                    pr,
		TriageBreached:        triageBreached,
		FirstReviewBreached:   firstReviewBreached,
		MergeDecisionBreached: mergeBreached,
		Age:                   age,
	}
}

// CheckAll evaluates a slice of PR records and returns results for all.
func (c *Checker) CheckAll(prs []PRRecord) []SLAResult {
	results := make([]SLAResult, 0, len(prs))
	for _, pr := range prs {
		results = append(results, c.Check(pr))
	}
	return results
}

// Breaches returns only the SLAResults that have at least one breach.
func Breaches(results []SLAResult) []SLAResult {
	var out []SLAResult
	for _, r := range results {
		if r.TriageBreached || r.FirstReviewBreached || r.MergeDecisionBreached {
			out = append(out, r)
		}
	}
	return out
}

// ── Snapshot I/O ──────────────────────────────────────────────────────────────

// WriteSnapshot serialises a slice of PRRecords as JSONL to w.
func WriteSnapshot(w io.Writer, prs []PRRecord) error {
	enc := json.NewEncoder(w)
	for _, pr := range prs {
		if err := enc.Encode(pr); err != nil {
			return errcode.Newf(ErrSLACheck, err, "write snapshot record PR#%d", pr.Number)
		}
	}
	return nil
}

// ReadSnapshot reads a JSONL snapshot file and returns the PR records.
func ReadSnapshot(path string) ([]PRRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errcode.Newf(ErrSLACheck, err, "open snapshot %s", path)
	}
	defer f.Close() //nolint:errcheck

	var prs []PRRecord
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var pr PRRecord
		if err := json.Unmarshal(line, &pr); err != nil {
			return nil, errcode.Newf(ErrSLACheck, err, "parse snapshot line")
		}
		prs = append(prs, pr)
	}
	if err := sc.Err(); err != nil {
		return nil, errcode.Newf(ErrSLACheck, err, "read snapshot %s", path)
	}
	return prs, nil
}

// ── Dashboard rendering ───────────────────────────────────────────────────────

// PrintDashboard writes a plain-text SLA dashboard to w.
func PrintDashboard(w io.Writer, results []SLAResult, policy SLAPolicy) {
	breached := Breaches(results)
	total := len(results)
	breachCount := len(breached)

	fmt.Fprintf(w, "=== Forge Maintainer Review-SLA Dashboard ===\n")
	fmt.Fprintf(w, "Policy: triage<%s  review<%s  merge<%s\n",
		policy.TriageWithin, policy.FirstReviewWithin, policy.MergeDecisionWithin)
	fmt.Fprintf(w, "Open PRs: %d  Breaching: %d\n\n", total, breachCount)

	if breachCount == 0 {
		fmt.Fprintln(w, "✓ All open PRs are within SLA targets.")
		return
	}

	for _, r := range breached {
		fmt.Fprintf(w, "  %s\n", r)
	}
	fmt.Fprintf(w, "\n%d SLA breach(es) require attention.\n", breachCount)
}
