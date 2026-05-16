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

package outbox_test

import (
	"testing"

	"github.com/teragrid/forge/internal/outbox"
)

// TestOutbox_EmitAndPending verifies Emit writes an event and PendingEvents returns it.
func TestOutbox_EmitAndPending(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ob, err := outbox.Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	key, err := ob.Emit("ship", "create spec", map[string]any{"slug": "auth-email"}, "corr-001")
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if key == "" {
		t.Error("Emit returned empty key")
	}
	pending, err := ob.PendingEvents()
	if err != nil {
		t.Fatalf("PendingEvents: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("PendingEvents: got %d, want 1", len(pending))
	}
	ev := pending[0]
	if ev.Verb != "ship" {
		t.Errorf("Verb = %q, want %q", ev.Verb, "ship")
	}
	if ev.Intent != "create spec" {
		t.Errorf("Intent = %q, want %q", ev.Intent, "create spec")
	}
	if ev.Status != outbox.StatusPending {
		t.Errorf("Status = %q, want pending", ev.Status)
	}
	if ev.CorrelationID != "corr-001" {
		t.Errorf("CorrelationID = %q, want %q", ev.CorrelationID, "corr-001")
	}
}

// TestOutbox_IdempotentEmit verifies duplicate Emit is a no-op.
func TestOutbox_IdempotentEmit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ob, err := outbox.Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	payload := map[string]any{"slug": "feature-x"}
	k1, err := ob.Emit("ship", "create spec", payload, "")
	if err != nil {
		t.Fatalf("first Emit: %v", err)
	}
	k2, err := ob.Emit("ship", "create spec", payload, "")
	if err != nil {
		t.Fatalf("second Emit: %v", err)
	}
	if k1 != k2 {
		t.Errorf("idempotency keys differ: %q vs %q", k1, k2)
	}
	pending, _ := ob.PendingEvents()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending event after duplicate Emit, got %d", len(pending))
	}
}

// TestOutbox_Advance transitions event status.
func TestOutbox_Advance(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ob, err := outbox.Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	key, err := ob.Emit("fix", "apply patch", nil, "")
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if err := ob.Advance(key, outbox.StatusProcessing); err != nil {
		t.Fatalf("Advance→processing: %v", err)
	}
	if err := ob.Advance(key, outbox.StatusDone); err != nil {
		t.Fatalf("Advance→done: %v", err)
	}
	// Should no longer appear in pending.
	pending, _ := ob.PendingEvents()
	for _, ev := range pending {
		if ev.IdempotencyKey == key {
			t.Error("advanced event still in pending")
		}
	}
}

// TestOutbox_Advance_MissingKey returns error.
func TestOutbox_Advance_MissingKey(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ob, err := outbox.Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := ob.Advance("nonexistent-key", outbox.StatusDone); err == nil {
		t.Error("expected error for missing key")
	}
}

// TestOutbox_EmptyDir returns empty pending.
func TestOutbox_EmptyDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ob, err := outbox.Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	pending, err := ob.PendingEvents()
	if err != nil {
		t.Fatalf("PendingEvents: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 pending, got %d", len(pending))
	}
}

// TestOutbox_MultiplePending returns multiple pending events.
func TestOutbox_MultiplePending(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ob, err := outbox.Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for i := 0; i < 3; i++ {
		_, err := ob.Emit("scan", "finding", map[string]any{"n": i}, "")
		if err != nil {
			t.Fatalf("Emit[%d]: %v", i, err)
		}
	}
	pending, err := ob.PendingEvents()
	if err != nil {
		t.Fatalf("PendingEvents: %v", err)
	}
	if len(pending) != 3 {
		t.Errorf("expected 3 pending, got %d", len(pending))
	}
}
