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

// immutable_audit.go — RFC-005 P3: Immutable audit trail verification.
//
// The existing audit.Ledger already chains SHA-256 hashes across entries
// (tamper-evident by design). This file adds:
//
//  1. VerifyLedger — re-computes every hash in the chain and reports any
//     entries whose stored hash does not match. A break indicates that the
//     file was modified or entries were deleted.
//
//  2. SealLedger / VerifySeal — appends an optional Ed25519 signature to a
//     separate <path>.sig file so the entire ledger can be verified by an
//     external party with access to the public key.
//
// Both functions are additive: they never modify the ledger itself.
package cmdship

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// AuditVerifyResult is returned by VerifyLedger.
type AuditVerifyResult struct {
	// Path is the ledger file that was verified.
	Path string `json:"path"`
	// TotalEntries is the number of entries read.
	TotalEntries int `json:"total_entries"`
	// Tampered is true when any entry fails the hash check.
	Tampered bool `json:"tampered"`
	// Breaks lists (0-based) entry indices where the chain is broken.
	Breaks []int `json:"breaks,omitempty"`
	// VerifiedAt is when the check ran.
	VerifiedAt time.Time `json:"verified_at"`
}

// auditEntry is the minimal JSON structure needed for hash recomputation.
// It mirrors audit.Entry without importing the audit package to keep cmdship
// self-contained.
type auditEntry struct {
	Timestamp time.Time         `json:"ts"`
	Verb      string            `json:"verb"`
	Action    string            `json:"action"`
	Actor     string            `json:"actor,omitempty"`
	Detail    map[string]string `json:"detail,omitempty"`
	PrevHash  string            `json:"prev_hash"`
	Hash      string            `json:"hash"`
}

// computeAuditHash replicates audit.computeHash logic:
// sha256(ts + verb + action + actor + prevHash).
func computeAuditHash(e auditEntry) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s|%s",
		e.Timestamp.UTC().Format(time.RFC3339Nano),
		e.Verb,
		e.Action,
		e.Actor,
		e.PrevHash,
	)
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyLedger reads every entry from the audit log at path, recomputes each
// hash, and reports chain breaks. Returns (result, nil) even when the chain
// is broken — the Tampered flag and Breaks list carry the findings.
func VerifyLedger(path string) (AuditVerifyResult, error) {
	result := AuditVerifyResult{
		Path:       path,
		VerifiedAt: time.Now().UTC(),
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return result, nil // empty ledger is valid
		}
		return result, fmt.Errorf("immutable audit: open: %w", err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	var prevHash string
	idx := 0
	for dec.More() {
		var e auditEntry
		if err := dec.Decode(&e); err != nil {
			return result, fmt.Errorf("immutable audit: decode entry %d: %w", idx, err)
		}
		// Verify PrevHash linkage.
		if e.PrevHash != prevHash {
			result.Breaks = append(result.Breaks, idx)
			result.Tampered = true
		}
		// Verify self-hash.
		want := computeAuditHash(e)
		if e.Hash != want {
			result.Breaks = append(result.Breaks, idx)
			result.Tampered = true
		}
		prevHash = e.Hash
		result.TotalEntries++
		idx++
	}
	return result, nil
}

// SealLedger reads the full audit log, computes an Ed25519 signature over its
// SHA-256 digest, and writes it to <path>.sig. The private key is provided by
// the caller — Forge never stores private keys.
func SealLedger(path string, privateKey ed25519.PrivateKey) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("immutable audit: read for seal: %w", err)
	}
	digest := sha256.Sum256(data)
	sig := ed25519.Sign(privateKey, digest[:])
	return os.WriteFile(path+".sig", sig, 0o600)
}

// VerifySeal checks that the .sig file for ledger at path was created by the
// holder of the private key corresponding to publicKey.
func VerifySeal(path string, publicKey ed25519.PublicKey) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("immutable audit: read for verify: %w", err)
	}
	sig, err := os.ReadFile(path + ".sig")
	if err != nil {
		return fmt.Errorf("immutable audit: read sig: %w", err)
	}
	digest := sha256.Sum256(data)
	if !ed25519.Verify(publicKey, digest[:], sig) {
		return fmt.Errorf("immutable audit: signature verification failed for %s", path)
	}
	return nil
}
