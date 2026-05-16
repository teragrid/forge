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

// Package tokenledger implements DEV-M0-18: an append-only per-request token
// cost ledger stored at .forge/token-ledger.jsonl. Each line is a JSON object
// (JSONL format) containing token usage and estimated cost for a single LLM
// call. The ledger provides aggregated summaries per model.
//
// File format (one JSON object per line):
//
//	{"time":"...","model":"...","input_tokens":10,"output_tokens":5,"cost_usd":0.00015,"operation":"..."}
//
// The Ledger is concurrency-safe; multiple goroutines may Append simultaneously.
package tokenledger

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/teragrid/forge/internal/errcode"
)

// Reserved error codes (range 2800..2899).
var (
	ErrWriteFail = errcode.Register(errcode.Code(2800), "token ledger write failed")
	ErrReadFail  = errcode.Register(errcode.Code(2801), "token ledger read failed")
)

// DefaultPath is the default ledger file location relative to the project root.
const DefaultPath = ".forge/token-ledger.jsonl"

// Entry records the token usage and cost for a single LLM invocation.
type Entry struct {
	Time         time.Time `json:"time"`
	Model        string    `json:"model"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CostUSD      float64   `json:"cost_usd"`
	Operation    string    `json:"operation"`
	// G-140: per-feature token attribution tags.
	Feature string `json:"feature,omitempty"` // slug of the forge ship feature (e.g. "auth-email")
	Actor   string `json:"actor,omitempty"`   // user / service account initiating the call
	Tenant  string `json:"tenant,omitempty"`  // workspace / org ID for multi-tenant attribution
}

// ModelSummary aggregates usage for a single model.
type ModelSummary struct {
	Model        string
	Calls        int
	InputTokens  int
	OutputTokens int
	TotalCostUSD float64
}

// FeatureSummary aggregates usage for a single feature slug (G-140).
type FeatureSummary struct {
	Feature      string
	Calls        int
	InputTokens  int
	OutputTokens int
	TotalCostUSD float64
}

// Summary holds the full cross-model aggregation.
type Summary struct {
	TotalCalls   int
	TotalCostUSD float64
	ByModel      map[string]*ModelSummary
	// G-140: per-feature cost breakdown.
	ByFeature map[string]*FeatureSummary
}

// Ledger is a thread-safe, append-only token usage ledger.
type Ledger struct {
	mu   sync.Mutex
	path string
}

// New returns a Ledger backed by path. Parent directories are created on first
// Append; New itself performs no I/O.
func New(path string) *Ledger {
	return &Ledger{path: path}
}

// Append writes one Entry to the ledger file. The file is opened, appended to,
// and closed on every call so that partial failures never corrupt earlier data.
func (l *Ledger) Append(e Entry) error {
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}

	data, err := json.Marshal(e)
	if err != nil {
		return errcode.New(ErrWriteFail, "failed to marshal entry", err)
	}
	data = append(data, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return errcode.New(ErrWriteFail, fmt.Sprintf("mkdir %s: %v", filepath.Dir(l.path), err), err)
	}

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return errcode.New(ErrWriteFail, fmt.Sprintf("open %s: %v", l.path, err), err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return errcode.New(ErrWriteFail, fmt.Sprintf("write to %s: %v", l.path, err), err)
	}
	return nil
}

// ReadAll returns all entries from the ledger file in append order.
// Returns an empty slice (not an error) if the file does not yet exist.
func (l *Ledger) ReadAll() ([]Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.Open(l.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errcode.New(ErrReadFail, fmt.Sprintf("open %s: %v", l.path, err), err)
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, errcode.New(ErrReadFail,
				fmt.Sprintf("parse error at line %d: %v", lineNo, err), err)
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, errcode.New(ErrReadFail, fmt.Sprintf("scan %s: %v", l.path, err), err)
	}
	return entries, nil
}

// TotalCost returns the sum of CostUSD across all entries.
func (l *Ledger) TotalCost() (float64, error) {
	entries, err := l.ReadAll()
	if err != nil {
		return 0, err
	}
	var total float64
	for _, e := range entries {
		total += e.CostUSD
	}
	return total, nil
}

// Summary returns a cross-model aggregation of all ledger entries.
func (l *Ledger) Summary() (*Summary, error) {
	entries, err := l.ReadAll()
	if err != nil {
		return nil, err
	}
	s := &Summary{
		ByModel:   make(map[string]*ModelSummary),
		ByFeature: make(map[string]*FeatureSummary),
	}
	for _, e := range entries {
		s.TotalCalls++
		s.TotalCostUSD += e.CostUSD
		ms, ok := s.ByModel[e.Model]
		if !ok {
			ms = &ModelSummary{Model: e.Model}
			s.ByModel[e.Model] = ms
		}
		ms.Calls++
		ms.InputTokens += e.InputTokens
		ms.OutputTokens += e.OutputTokens
		ms.TotalCostUSD += e.CostUSD
		// G-140: per-feature attribution.
		if e.Feature != "" {
			fs, ok := s.ByFeature[e.Feature]
			if !ok {
				fs = &FeatureSummary{Feature: e.Feature}
				s.ByFeature[e.Feature] = fs
			}
			fs.Calls++
			fs.InputTokens += e.InputTokens
			fs.OutputTokens += e.OutputTokens
			fs.TotalCostUSD += e.CostUSD
		}
	}
	return s, nil
}

// DailySpend returns the total CostUSD recorded in the ledger for the
// calendar day of t (UTC).
func (l *Ledger) DailySpend(t time.Time) (float64, error) {
	entries, err := l.ReadAll()
	if err != nil {
		return 0, err
	}
	y, mo, d := t.UTC().Date()
	var total float64
	for _, e := range entries {
		ey, emo, ed := e.Time.UTC().Date()
		if ey == y && emo == mo && ed == d {
			total += e.CostUSD
		}
	}
	return total, nil
}

// DailyBudgetAlert returns a non-nil error when the ledger spend on the
// calendar day of t (UTC) meets or exceeds limitUSD. A zero limitUSD is
// treated as unlimited (always returns nil).
func (l *Ledger) DailyBudgetAlert(t time.Time, limitUSD float64) error {
	if limitUSD <= 0 {
		return nil
	}
	spent, err := l.DailySpend(t)
	if err != nil {
		return err
	}
	if spent >= limitUSD {
		return fmt.Errorf("tokenledger: daily spend USD%.4f meets/exceeds limit USD%.4f", spent, limitUSD)
	}
	return nil
}
