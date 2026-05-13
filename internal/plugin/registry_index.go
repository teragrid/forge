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

// Package plugin — registry_index.go provides a local, signed JSON index of
// well-known community plugins (M2-01).
//
// The index is embedded at build time and consulted by `forge plugin list
// --remote` and `forge plugin search`. Remote fetch + signature verification
// is planned for M2.2; this file holds the bootstrapping catalog.
package plugin

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed registry_index.json
var registryIndexJSON []byte

// IndexEntry describes a community plugin in the registry catalog.
type IndexEntry struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"` // scanner | codemod | provider | template
	Version   string   `json:"version"`
	Author    string   `json:"author"`
	Summary   string   `json:"summary"`
	SourceURL string   `json:"source_url"`
	SHA256    string   `json:"sha256"`
	Tags      []string `json:"tags"`
	Forge     string   `json:"forge"` // minimum Forge version
}

// RegistryIndex is the top-level structure of registry_index.json.
type RegistryIndex struct {
	SchemaVersion string       `json:"schema_version"`
	GeneratedAt   string       `json:"generated_at"`
	Entries       []IndexEntry `json:"entries"`
}

// LoadRegistryIndex parses and returns the embedded plugin registry catalog.
func LoadRegistryIndex() (RegistryIndex, error) {
	var idx RegistryIndex
	if err := json.Unmarshal(registryIndexJSON, &idx); err != nil {
		return RegistryIndex{}, fmt.Errorf("parse registry index: %w", err)
	}
	return idx, nil
}

// SearchIndex returns all IndexEntry records whose Name, Summary, or Tags
// contain the given query string (case-insensitive).
func SearchIndex(query string) ([]IndexEntry, error) {
	idx, err := LoadRegistryIndex()
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var results []IndexEntry
	for _, e := range idx.Entries {
		if strings.Contains(strings.ToLower(e.Name), q) ||
			strings.Contains(strings.ToLower(e.Summary), q) {
			results = append(results, e)
			continue
		}
		for _, t := range e.Tags {
			if strings.Contains(strings.ToLower(t), q) {
				results = append(results, e)
				break
			}
		}
	}
	return results, nil
}
