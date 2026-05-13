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

// aggregator.go — local learning-loop aggregator MVP (M2-10).
//
// The Aggregator collects Event records on-disk, computes simple statistics
// (error-rate by verb, p50/p95 token counts), and can produce a summary
// report. It intentionally runs entirely locally — no external service is
// required for the MVP.
//
// Future work: wire the aggregator into a community endpoint so teams can
// share aggregated statistics (not raw events) with the Forge project.
package learningloop

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// AggregateRecord stores all events received by the aggregator.
type AggregateRecord struct {
	Events    []Event   `json:"events"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Aggregator collects and analyses learning-loop events.
type Aggregator struct {
	storePath string
}

// NewAggregator returns an Aggregator that persists data to
// root/.forge/learn-aggregate.json.
func NewAggregator(root string) *Aggregator {
	return &Aggregator{
		storePath: filepath.Join(root, ".forge", "learn-aggregate.json"),
	}
}

// Ingest appends events to the aggregate store.
func (a *Aggregator) Ingest(events []Event) error {
	rec, err := a.load()
	if err != nil {
		return err
	}
	rec.Events = append(rec.Events, events...)
	rec.UpdatedAt = time.Now().UTC()
	return a.save(rec)
}

// Stats holds computed statistics over the aggregate.
type Stats struct {
	TotalEvents int                  `json:"total_events"`
	ByVerb      map[string]VerbStats `json:"by_verb"`
	OverallP50  int                  `json:"overall_p50_tokens"`
	OverallP95  int                  `json:"overall_p95_tokens"`
	ComputedAt  time.Time            `json:"computed_at"`
}

// VerbStats holds per-verb statistics.
type VerbStats struct {
	Count     int     `json:"count"`
	ErrorRate float64 `json:"error_rate"`
	P50Tokens int     `json:"p50_tokens"`
	P95Tokens int     `json:"p95_tokens"`
}

// Compute derives Stats from all stored events.
func (a *Aggregator) Compute() (*Stats, error) {
	rec, err := a.load()
	if err != nil {
		return nil, err
	}

	stats := &Stats{
		TotalEvents: len(rec.Events),
		ByVerb:      map[string]VerbStats{},
		ComputedAt:  time.Now().UTC(),
	}

	byVerb := map[string][]Event{}
	var allTokens []int
	for _, e := range rec.Events {
		byVerb[e.Verb] = append(byVerb[e.Verb], e)
		allTokens = append(allTokens, e.InputTokens+e.OutputTokens)
	}

	for verb, events := range byVerb {
		var errs int
		var tokens []int
		for _, e := range events {
			if e.Outcome == "error" {
				errs++
			}
			tokens = append(tokens, e.InputTokens+e.OutputTokens)
		}
		sort.Ints(tokens)
		stats.ByVerb[verb] = VerbStats{
			Count:     len(events),
			ErrorRate: float64(errs) / float64(len(events)),
			P50Tokens: percentile(tokens, 50),
			P95Tokens: percentile(tokens, 95),
		}
	}

	sort.Ints(allTokens)
	stats.OverallP50 = percentile(allTokens, 50)
	stats.OverallP95 = percentile(allTokens, 95)
	return stats, nil
}

// Report returns a human-readable text summary.
func (a *Aggregator) Report() (string, error) {
	stats, err := a.Compute()
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Learning Loop Aggregate (computed at %s):\n%s\n",
		stats.ComputedAt.Format(time.RFC3339), string(data)), nil
}

func (a *Aggregator) load() (*AggregateRecord, error) {
	data, err := os.ReadFile(a.storePath)
	if os.IsNotExist(err) {
		return &AggregateRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	var rec AggregateRecord
	return &rec, json.Unmarshal(data, &rec)
}

func (a *Aggregator) save(rec *AggregateRecord) error {
	if err := os.MkdirAll(filepath.Dir(a.storePath), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.storePath, data, 0o600)
}

// percentile returns the pth percentile value from a pre-sorted slice.
func percentile(sorted []int, p int) int {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(float64(p)/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
