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

// Package audit – twokey.go implements the ADR-022 two-key enforcement
// protocol (M3-20). High-impact operations (secret rotation, plugin removal,
// release signing, audit-log deletion) require approval from a second
// authorised key before execution.
//
// The two-key gate works as follows:
//  1. The initiating operator creates a TwoKeyRequest and stores it in
//     .forge/twokey/<op-id>.json (signed with their key).
//  2. A second operator runs `forge twokey approve <op-id>` which adds their
//     signature to the same file.
//  3. The original command continues only when both signatures are present.
//     It records a TwoKeyEntry in the audit ledger.
package audit

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// HighImpactOps is the allow-list of operations that require two-key approval.
// Each string matches the Action field used in audit.Entry.
var HighImpactOps = []string{
	"secret.rotate",
	"plugin.remove",
	"release.sign",
	"audit.delete",
	"config.wipe",
}

// IsHighImpact returns true if action requires two-key approval.
func IsHighImpact(action string) bool {
	for _, op := range HighImpactOps {
		if op == action {
			return true
		}
	}
	return false
}

// TwoKeyRequest is the on-disk record for a pending two-key approval.
type TwoKeyRequest struct {
	SchemaVersion string            `json:"schema_version"`
	ID            string            `json:"id"`
	Action        string            `json:"action"`
	Detail        map[string]string `json:"detail,omitempty"`
	InitiatorKey  string            `json:"initiator_key"` // hex-encoded Ed25519 public key
	InitiatorSig  string            `json:"initiator_sig"` // hex-encoded signature over canonical payload
	ApproverKey   string            `json:"approver_key"`  // filled by second operator
	ApproverSig   string            `json:"approver_sig"`  // filled by second operator
	CreatedAt     string            `json:"created_at"`
	ApprovedAt    string            `json:"approved_at,omitempty"`
}

// DefaultTwoKeyDir is where pending requests are stored.
const DefaultTwoKeyDir = ".forge/twokey"

// canonicalPayload returns the deterministic bytes that both parties sign.
// Only stable fields are included; sigs and approved_at are excluded.
func canonicalPayload(req *TwoKeyRequest) ([]byte, error) {
	payload := map[string]any{
		"id":         req.ID,
		"action":     req.Action,
		"detail":     req.Detail,
		"created_at": req.CreatedAt,
	}
	return json.Marshal(payload)
}

// NewTwoKeyRequest creates a new pending request, signing it with privKey.
// privKey must be an Ed25519 private key (64 bytes).
func NewTwoKeyRequest(action string, detail map[string]string, privKey ed25519.PrivateKey) (*TwoKeyRequest, error) {
	if !IsHighImpact(action) {
		return nil, fmt.Errorf("twokey: %q is not a high-impact operation", action)
	}

	id, err := randomHex(8)
	if err != nil {
		return nil, fmt.Errorf("twokey: generate id: %w", err)
	}

	req := &TwoKeyRequest{
		SchemaVersion: "1",
		ID:            id,
		Action:        action,
		Detail:        detail,
		InitiatorKey:  hex.EncodeToString(privKey.Public().(ed25519.PublicKey)),
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	payload, err := canonicalPayload(req)
	if err != nil {
		return nil, fmt.Errorf("twokey: canonical payload: %w", err)
	}
	sig := ed25519.Sign(privKey, payload)
	req.InitiatorSig = hex.EncodeToString(sig)
	return req, nil
}

// Approve adds the approver's signature to req.
// privKey must be a different key than the initiator's.
func Approve(req *TwoKeyRequest, privKey ed25519.PrivateKey) error {
	pubHex := hex.EncodeToString(privKey.Public().(ed25519.PublicKey))
	if pubHex == req.InitiatorKey {
		return fmt.Errorf("twokey: initiator and approver must be different keys")
	}
	if req.ApproverKey != "" {
		return fmt.Errorf("twokey: request %s already approved", req.ID)
	}

	payload, err := canonicalPayload(req)
	if err != nil {
		return fmt.Errorf("twokey: canonical payload: %w", err)
	}
	sig := ed25519.Sign(privKey, payload)
	req.ApproverKey = pubHex
	req.ApproverSig = hex.EncodeToString(sig)
	req.ApprovedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// Verify checks that both signatures are valid and that the keys differ.
// Returns nil if the request may proceed.
func Verify(req *TwoKeyRequest) error {
	if req.InitiatorKey == "" || req.InitiatorSig == "" {
		return fmt.Errorf("twokey: missing initiator key/sig")
	}
	if req.ApproverKey == "" || req.ApproverSig == "" {
		return fmt.Errorf("twokey: request %s not yet approved", req.ID)
	}
	if req.InitiatorKey == req.ApproverKey {
		return fmt.Errorf("twokey: initiator and approver must be different keys")
	}

	payload, err := canonicalPayload(req)
	if err != nil {
		return err
	}

	initPub, err := decodeEd25519Pub(req.InitiatorKey)
	if err != nil {
		return fmt.Errorf("twokey: decode initiator key: %w", err)
	}
	initSig, err := hex.DecodeString(req.InitiatorSig)
	if err != nil {
		return fmt.Errorf("twokey: decode initiator sig: %w", err)
	}
	if !ed25519.Verify(initPub, payload, initSig) {
		return fmt.Errorf("twokey: initiator signature invalid for request %s", req.ID)
	}

	appPub, err := decodeEd25519Pub(req.ApproverKey)
	if err != nil {
		return fmt.Errorf("twokey: decode approver key: %w", err)
	}
	appSig, err := hex.DecodeString(req.ApproverSig)
	if err != nil {
		return fmt.Errorf("twokey: decode approver sig: %w", err)
	}
	if !ed25519.Verify(appPub, payload, appSig) {
		return fmt.Errorf("twokey: approver signature invalid for request %s", req.ID)
	}

	return nil
}

// SaveRequest writes req to dir/<id>.json (mode 0600).
func SaveRequest(dir string, req *TwoKeyRequest) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("twokey: mkdir %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return fmt.Errorf("twokey: marshal: %w", err)
	}
	path := filepath.Join(dir, req.ID+".json")
	return os.WriteFile(path, data, 0o600)
}

// LoadRequest reads a TwoKeyRequest from dir/<id>.json.
func LoadRequest(dir, id string) (*TwoKeyRequest, error) {
	path := filepath.Join(dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("twokey: read %s: %w", path, err)
	}
	var req TwoKeyRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("twokey: parse %s: %w", path, err)
	}
	return &req, nil
}

// decodeEd25519Pub decodes a hex-encoded Ed25519 public key.
func decodeEd25519Pub(hexKey string) (ed25519.PublicKey, error) {
	b, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("hex decode: %w", err)
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("expected %d bytes, got %d", ed25519.PublicKeySize, len(b))
	}
	return ed25519.PublicKey(b), nil
}

// randomHex returns n random bytes as a hex string.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
