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

// G-102: Emit() helper — convenience function wired into every generated
// mutation: actor, intent, before/after diff, correlation ID.
package audit

import (
	"fmt"
	"os"
	"path/filepath"
)

// MutationEvent carries structured fields for a mutation audit emission.
type MutationEvent struct {
	// Verb is the forge verb performing the mutation (e.g. "ship", "generate").
	Verb string
	// Intent is a human-readable description of the mutation intent.
	Intent string
	// Actor is who initiated the mutation (defaults to OS user if empty).
	Actor string
	// Before is a text summary of state before the mutation.
	Before string
	// After is a text summary of state after the mutation.
	After string
	// CorrelationID links related mutations in the same pipeline run.
	CorrelationID string
}

// Emit appends a MutationEvent to the default audit ledger at <root>/.forge/audit.log.
// Generated modules call this on every write path.
func Emit(root string, ev MutationEvent) error {
	actor := ev.Actor
	if actor == "" {
		if u, err := osUser(); err == nil {
			actor = u
		}
	}
	detail := map[string]string{
		"intent": ev.Intent,
		"before": ev.Before,
		"after":  ev.After,
	}
	if ev.CorrelationID != "" {
		detail["correlation_id"] = ev.CorrelationID
	}
	ledger, err := Open(filepath.Join(root, DefaultPath))
	if err != nil {
		return fmt.Errorf("audit.Emit open ledger: %w", err)
	}
	_, err = ledger.Append(Entry{
		Verb:   ev.Verb,
		Action: ev.Intent,
		Actor:  actor,
		Detail: detail,
	})
	return err
}

// osUser returns the OS username for the current process.
func osUser() (string, error) {
	if u := os.Getenv("USER"); u != "" {
		return u, nil
	}
	if u := os.Getenv("USERNAME"); u != "" {
		return u, nil
	}
	return "", fmt.Errorf("no USER/USERNAME env var")
}
