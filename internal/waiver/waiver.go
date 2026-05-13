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

// Package waiver implements DEV-M1-17: the .forge/waivers/ directory reader
// and waiver evaluation logic for the scan engine.
//
// A waiver exempts a specific finding (identified by rule_id + optionally
// file:line) from failing scans. Waivers are YAML files under .forge/waivers/
// with the schema defined by WaiverSpec.
//
// Expiry: waivers past their ExpiresAt date are NEVER honoured (strict
// enforcement). The scan engine calls IsWaived() and acts on the result;
// expired waivers produce an ErrWaiverExpired error from IsWaived().
package waiver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultDir is the project-relative path for waiver files.
const DefaultDir = ".forge/waivers"

// ErrWaiverExpired is returned when a matching waiver exists but has expired.
var ErrWaiverExpired = errors.New("waiver: entry has expired")

// WaiverSpec is the on-disk YAML schema for a single waiver file.
// A file may contain multiple specs as a YAML list.
type WaiverSpec struct {
	// ID is a stable human-assigned identifier (e.g. W-001).
	ID string `yaml:"id"`

	// RuleID is the scan rule this waiver covers (e.g. "SEC-001").
	RuleID string `yaml:"rule_id"`

	// FilePath, if set, scopes the waiver to a specific file.
	FilePath string `yaml:"file_path,omitempty"`

	// Rationale is a required human-readable justification.
	Rationale string `yaml:"rationale"`

	// ApprovedBy is the name/GH handle of the approver.
	ApprovedBy string `yaml:"approved_by"`

	// ExpiresAt is the date after which this waiver is no longer valid.
	// Format: "YYYY-MM-DD"
	ExpiresAt string `yaml:"expires_at"`

	// CreatedAt is when this waiver was created (informational).
	CreatedAt string `yaml:"created_at,omitempty"`
}

// Expired returns true if this waiver is past its expiry date.
func (w WaiverSpec) Expired() bool {
	if w.ExpiresAt == "" {
		return false
	}
	t, err := time.Parse("2006-01-02", w.ExpiresAt)
	if err != nil {
		return false // malformed date → not expired (safe default)
	}
	return time.Now().UTC().After(t.UTC().AddDate(0, 0, 1)) // expired after end of ExpiresAt day
}

// Registry holds all loaded waivers for a project.
type Registry struct {
	waivers []WaiverSpec
	dir     string
}

// Load reads all *.yml / *.yaml waiver files from dir and returns a Registry.
func Load(dir string) (*Registry, error) {
	r := &Registry{dir: dir}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil // no waiver directory is fine
		}
		return nil, fmt.Errorf("waiver: read dir %s: %w", dir, err)
	}
	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("waiver: read %s: %w", path, err)
		}
		var specs []WaiverSpec
		if err := yaml.Unmarshal(data, &specs); err != nil {
			// Try single-document (not a list).
			var single WaiverSpec
			if err2 := yaml.Unmarshal(data, &single); err2 != nil {
				return nil, fmt.Errorf("waiver: parse %s: %w", path, err)
			}
			specs = []WaiverSpec{single}
		}
		r.waivers = append(r.waivers, specs...)
	}
	return r, nil
}

// LoadDefault loads from the default waiver directory under root.
func LoadDefault(root string) (*Registry, error) {
	return Load(filepath.Join(root, DefaultDir))
}

// IsWaived returns true if the finding (ruleID + filePath) is covered by
// a valid, non-expired waiver.
//
// filePath may be empty to match any file.
// Returns ErrWaiverExpired if a matching waiver was found but is expired
// (caller should surface this as an error, not silently pass the finding).
func (r *Registry) IsWaived(ruleID, filePath string) (bool, error) {
	for _, w := range r.waivers {
		if w.RuleID != ruleID {
			continue
		}
		// FilePath match (empty waiver.FilePath matches any file).
		if w.FilePath != "" && w.FilePath != filePath {
			continue
		}
		if w.Expired() {
			return false, fmt.Errorf("%w: rule=%s id=%s expires=%s", ErrWaiverExpired, ruleID, w.ID, w.ExpiresAt)
		}
		return true, nil
	}
	return false, nil
}

// All returns all loaded waivers (for listing/reporting).
func (r *Registry) All() []WaiverSpec { return r.waivers }

// ActiveCount returns the number of non-expired waivers.
func (r *Registry) ActiveCount() int {
	n := 0
	for _, w := range r.waivers {
		if !w.Expired() {
			n++
		}
	}
	return n
}

// ExpiredCount returns the number of expired waivers.
func (r *Registry) ExpiredCount() int {
	n := 0
	for _, w := range r.waivers {
		if w.Expired() {
			n++
		}
	}
	return n
}
