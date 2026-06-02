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

// ab_steering.go — RFC-005 P3: A/B steering experiment harness.
//
// ABExperiment lets the ship pipeline compare two steering prompt variants
// (A and B) for the same checkpoint and record which achieves a higher
// TestScoreResult composite score. Results are appended as JSONL to
// .forge/ab-experiments/<name>.jsonl.
//
// Design:
//   - Each experiment has a unique name (e.g. "arch-review-style").
//   - Variant A is the current/control; Variant B is the challenger.
//   - The caller supplies a ScoreFunc that evaluates the LLM response.
//   - ABExperiment.Run dispatches both variants and records which wins.
//   - ABReport summarizes accumulated wins/losses across all runs.
//
// Experiments are opt-in and never block the pipeline; if scoring fails the
// result is recorded as "inconclusive" and the pipeline continues normally.
package cmdship

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/teragrid/forge/internal/errcode"
)

// ErrABVariantConflict is returned when an experiment name is registered twice
// with conflicting variant definitions.
var ErrABVariantConflict = errcode.Register(errcode.Code(3216),
	"A/B steering variant collision — experiment name already registered with different variants")

// ABVariantID identifies which variant was used in an experiment run.
type ABVariantID string

const (
	ABVariantA ABVariantID = "A"
	ABVariantB ABVariantID = "B"
)

// ABExperimentDef defines an A/B experiment comparing two steering prompts.
type ABExperimentDef struct {
	// Name uniquely identifies the experiment (used as the JSONL filename stem).
	Name string
	// VariantA is the control steering prompt.
	VariantA string
	// VariantB is the challenger steering prompt.
	VariantB string
	// Checkpoint is the pipeline checkpoint this experiment targets.
	Checkpoint string
}

// ABRunRecord is one completed experiment run appended to the JSONL log.
type ABRunRecord struct {
	ExperimentName string      `json:"experiment_name"`
	Checkpoint     string      `json:"checkpoint"`
	RunAt          time.Time   `json:"run_at"`
	Winner         ABVariantID `json:"winner"` // "A" | "B" | "tie" | "inconclusive"
	ScoreA         float64     `json:"score_a"`
	ScoreB         float64     `json:"score_b"`
	Note           string      `json:"note,omitempty"`
}

// ABReport is the aggregate summary for one experiment name.
type ABReport struct {
	ExperimentName string  `json:"experiment_name"`
	TotalRuns      int     `json:"total_runs"`
	WinsA          int     `json:"wins_a"`
	WinsB          int     `json:"wins_b"`
	Ties           int     `json:"ties"`
	Inconclusive   int     `json:"inconclusive"`
	MeanScoreA     float64 `json:"mean_score_a"`
	MeanScoreB     float64 `json:"mean_score_b"`
}

// abExperimentsDir returns the directory where experiment JSONL files live.
func abExperimentsDir(root string) string {
	return filepath.Join(root, ".forge", "ab-experiments")
}

// abExperimentPath returns the JSONL file path for one experiment.
func abExperimentPath(root, name string) string {
	return filepath.Join(abExperimentsDir(root), name+".jsonl")
}

// RecordABRun appends one ABRunRecord to the experiment's JSONL log.
// The directory is created on first write.
func RecordABRun(root string, rec ABRunRecord) error {
	dir := abExperimentsDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ab steering: mkdir: %w", err)
	}
	f, err := os.OpenFile(abExperimentPath(root, rec.ExperimentName),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("ab steering: open: %w", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(rec)
}

// RunABExperiment dispatches Variant A and B prompts via pipe, scores both
// with scoreFunc, records the result, and returns the winning variant ID.
//
// scoreFunc receives the LLM response text and returns a score in [0,10].
// If scoreFunc panics or returns an error the run is recorded as inconclusive.
// A nil pipe records an inconclusive run (dry-run / no LLM provider).
func RunABExperiment(
	root string,
	def ABExperimentDef,
	pipe *LLMPipe,
	systemBase, userPrompt string,
	maxTokens int,
	scoreFunc func(response string) float64,
) (ABVariantID, error) {
	rec := ABRunRecord{
		ExperimentName: def.Name,
		Checkpoint:     def.Checkpoint,
		RunAt:          time.Now().UTC(),
		Winner:         "inconclusive",
	}
	if pipe == nil {
		rec.Note = "no LLM provider — dry-run"
		_ = RecordABRun(root, rec)
		return "inconclusive", nil
	}

	respA, err := pipe.Invoke(def.Name+".A", "", systemBase+"\n\n"+def.VariantA, userPrompt, maxTokens)
	if err != nil {
		rec.Note = "variant A invoke failed: " + err.Error()
		_ = RecordABRun(root, rec)
		return "inconclusive", nil
	}
	respB, err := pipe.Invoke(def.Name+".B", "", systemBase+"\n\n"+def.VariantB, userPrompt, maxTokens)
	if err != nil {
		rec.Note = "variant B invoke failed: " + err.Error()
		_ = RecordABRun(root, rec)
		return "inconclusive", nil
	}

	rec.ScoreA = scoreFunc(respA)
	rec.ScoreB = scoreFunc(respB)

	switch {
	case rec.ScoreA > rec.ScoreB:
		rec.Winner = ABVariantA
	case rec.ScoreB > rec.ScoreA:
		rec.Winner = ABVariantB
	default:
		rec.Winner = "tie"
	}
	if err := RecordABRun(root, rec); err != nil {
		return rec.Winner, fmt.Errorf("ab steering: record: %w", err)
	}
	return rec.Winner, nil
}

// GetABReport reads all runs for experimentName and returns an aggregate report.
// Returns an empty report (not an error) when no runs have been recorded yet.
func GetABReport(root, experimentName string) (ABReport, error) {
	path := abExperimentPath(root, experimentName)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ABReport{ExperimentName: experimentName}, nil
		}
		return ABReport{}, fmt.Errorf("ab steering: open report: %w", err)
	}
	defer f.Close()

	rep := ABReport{ExperimentName: experimentName}
	var sumA, sumB float64
	dec := json.NewDecoder(f)
	for dec.More() {
		var rec ABRunRecord
		if err := dec.Decode(&rec); err != nil {
			continue
		}
		rep.TotalRuns++
		sumA += rec.ScoreA
		sumB += rec.ScoreB
		switch rec.Winner {
		case ABVariantA:
			rep.WinsA++
		case ABVariantB:
			rep.WinsB++
		case "tie":
			rep.Ties++
		default:
			rep.Inconclusive++
		}
	}
	if rep.TotalRuns > 0 {
		rep.MeanScoreA = sumA / float64(rep.TotalRuns)
		rep.MeanScoreB = sumB / float64(rep.TotalRuns)
	}
	return rep, nil
}
