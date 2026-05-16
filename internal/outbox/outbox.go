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

// Package outbox implements G-101: transactional outbox + idempotency keys.
//
// Every generated mutation writes an OutboxEvent before returning so that
// downstream processors can replay failures without double-applying changes.
// The idempotency key prevents double-processing if the same event is
// delivered more than once.
//
// On-disk layout: .forge/outbox/<idempotency-key>.json per event.
// Status transitions: pending → processing → done | failed.
package outbox

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// EventStatus represents the lifecycle state of an outbox event.
type EventStatus string

const (
	StatusPending    EventStatus = "pending"
	StatusProcessing EventStatus = "processing"
	StatusDone       EventStatus = "done"
	StatusFailed     EventStatus = "failed"
)

// Event is one durable event record written before a mutation.
type Event struct {
	// IdempotencyKey is a content-addressed hash that prevents double-processing.
	IdempotencyKey string `json:"idempotency_key"`
	// Verb is the forge verb that produced this event.
	Verb string `json:"verb"`
	// Intent describes the mutation being performed.
	Intent string `json:"intent"`
	// Payload is the mutation input (arbitrary JSON).
	Payload map[string]any `json:"payload,omitempty"`
	// CorrelationID links events from the same pipeline run.
	CorrelationID string `json:"correlation_id,omitempty"`
	// Status is the current lifecycle state.
	Status EventStatus `json:"status"`
	// CreatedAt is when this event was written.
	CreatedAt string `json:"created_at"`
	// UpdatedAt is when this event was last updated.
	UpdatedAt string `json:"updated_at,omitempty"`
}

// Outbox is a file-backed transactional outbox.
type Outbox struct {
	dir string
}

// Open opens (or creates) the outbox directory.
func Open(root string) (*Outbox, error) {
	dir := filepath.Join(root, ".forge", "outbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("outbox: mkdir %s: %w", dir, err)
	}
	return &Outbox{dir: dir}, nil
}

// Emit writes an Event with StatusPending. Returns the idempotency key.
// If an event with the same key already exists, it is a no-op (idempotent).
func (o *Outbox) Emit(verb, intent string, payload map[string]any, correlationID string) (string, error) {
	key := idempotencyKey(verb, intent, payload)
	path := filepath.Join(o.dir, key+".json")

	// Idempotency check — if event already exists, do not overwrite.
	if _, err := os.Stat(path); err == nil {
		return key, nil // already emitted
	}

	now := time.Now().UTC().Format(time.RFC3339)
	event := Event{
		IdempotencyKey: key,
		Verb:           verb,
		Intent:         intent,
		Payload:        payload,
		CorrelationID:  correlationID,
		Status:         StatusPending,
		CreatedAt:      now,
	}
	data, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return "", fmt.Errorf("outbox emit: marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("outbox emit: write: %w", err)
	}
	return key, nil
}

// Advance updates the status of an event identified by its idempotency key.
func (o *Outbox) Advance(key string, status EventStatus) error {
	path := filepath.Join(o.dir, key+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("outbox advance: read %s: %w", key, err)
	}
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("outbox advance: parse %s: %w", key, err)
	}
	event.Status = status
	event.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	updated, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, updated, 0o600)
}

// PendingEvents returns all events in StatusPending.
func (o *Outbox) PendingEvents() ([]Event, error) {
	return o.eventsWithStatus(StatusPending)
}

func (o *Outbox) eventsWithStatus(status EventStatus) ([]Event, error) {
	entries, err := os.ReadDir(o.dir)
	if err != nil {
		return nil, fmt.Errorf("outbox list: %w", err)
	}
	var events []Event
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(o.dir, e.Name()))
		if err != nil {
			continue
		}
		var ev Event
		if err := json.Unmarshal(data, &ev); err != nil {
			continue
		}
		if ev.Status == status {
			events = append(events, ev)
		}
	}
	return events, nil
}

// idempotencyKey returns a stable SHA-256 hash of (verb+intent+payload).
func idempotencyKey(verb, intent string, payload map[string]any) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00", verb, intent)
	if payload != nil {
		if data, err := json.Marshal(payload); err == nil {
			h.Write(data)
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16] // 16-char prefix is unique enough
}
