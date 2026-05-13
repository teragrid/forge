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
// Package audit implements an append-only ledger of forge actions
// (DEV-M0-08, M2 hardening). Each entry is a JSON line written to
// `.forge/audit.log`. The ledger is content-addressed: every entry
// includes a sha256 of the previous entry, forming a tamper-evident chain.
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultPath is where ledger entries are appended.
const DefaultPath = ".forge/audit.log"

// Entry is one immutable record in the ledger.
type Entry struct {
	Timestamp time.Time         `json:"ts"`
	Verb      string            `json:"verb"`
	Action    string            `json:"action"`
	Actor     string            `json:"actor,omitempty"`
	Detail    map[string]string `json:"detail,omitempty"`
	PrevHash  string            `json:"prev_hash"`
	Hash      string            `json:"hash"`
}

// Ledger is an append-only, hash-chained log.
type Ledger struct {
	mu       sync.Mutex
	path     string
	lastHash string
}

// Open returns a Ledger backed by path. Creates parent dirs as needed.
// Reads any existing file to recover the previous hash.
func Open(path string) (*Ledger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("audit: mkdir: %w", err)
	}
	l := &Ledger{path: path}
	if err := l.recoverLastHash(); err != nil {
		return nil, err
	}
	return l, nil
}

// Append writes a new entry with the chained hash. Concurrency-safe.
func (l *Ledger) Append(e Entry) (Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	e.PrevHash = l.lastHash
	e.Hash = computeHash(e)

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return Entry{}, fmt.Errorf("audit: open: %w", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	if err := enc.Encode(e); err != nil {
		return Entry{}, fmt.Errorf("audit: encode: %w", err)
	}
	l.lastHash = e.Hash
	return e, nil
}

// All reads every entry from the ledger.
func (l *Ledger) All() ([]Entry, error) {
	f, err := os.Open(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	return parseEntries(f)
}

// Verify walks the chain from genesis and confirms every hash matches.
// Returns the index of the first broken entry (or -1 if intact).
func (l *Ledger) Verify() (int, error) {
	entries, err := l.All()
	if err != nil {
		return -1, err
	}
	prev := ""
	for i, e := range entries {
		if e.PrevHash != prev {
			return i, fmt.Errorf("audit: prev_hash mismatch at #%d", i)
		}
		got := computeHash(e)
		if e.Hash != got {
			return i, fmt.Errorf("audit: hash mismatch at #%d (got %s, want %s)", i, got, e.Hash)
		}
		prev = e.Hash
	}
	return -1, nil
}

func (l *Ledger) recoverLastHash() error {
	entries, err := l.All()
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		l.lastHash = entries[len(entries)-1].Hash
	}
	return nil
}

func parseEntries(r io.Reader) ([]Entry, error) {
	dec := json.NewDecoder(r)
	var out []Entry
	for {
		var e Entry
		if err := dec.Decode(&e); err == io.EOF {
			return out, nil
		} else if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
}

func computeHash(e Entry) string {
	// Hash the canonical JSON of the entry MINUS its own hash field.
	cp := e
	cp.Hash = ""
	b, _ := json.Marshal(cp)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
