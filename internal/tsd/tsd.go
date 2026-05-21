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

// Package tsd implements the Tech Stack Decision (TSD) schema parser,
// validator, and module resolver used by `forge new` TSD mode.
// Schema version 1 is defined in private/docs/TEMPLATE_ENHANCEMENT_SPEC.md §3.1.
package tsd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/teragrid/forge/internal/errcode"
)

// ErrUnsupportedVersion is returned when tsd_version is not 1.
var ErrUnsupportedVersion = errcode.Register(errcode.Code(6400), "unsupported TSD schema version")

// knownTopKeys are the only valid top-level keys in a TSD document.
var knownTopKeys = map[string]bool{
	"tsd_version": true,
	"project":     true,
	"stack":       true,
}

// knownStackKeys are the only valid keys under the `stack:` block.
var knownStackKeys = map[string]bool{
	"frontend":      true,
	"backend":       true,
	"database":      true,
	"ai":            true,
	"payments":      true,
	"messaging":     true,
	"infra":         true,
	"observability": true,
	"compliance":    true,
}

// TSD is the in-memory representation of a .forge/tsd.yml file.
type TSD struct {
	TSDVersion  int      `yaml:"tsd_version"`
	Project     Project  `yaml:"project"`
	Stack       Stack    `yaml:"stack"`
	UnknownKeys []string `yaml:"-"` // keys not in the schema — warnings only
}

// Project holds project-level metadata.
type Project struct {
	Name        string `yaml:"name"`
	Domain      string `yaml:"domain"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
}

// Stack holds all technology stack choices.
type Stack struct {
	Frontend      Frontend      `yaml:"frontend"`
	Backend       Backend       `yaml:"backend"`
	Database      Database      `yaml:"database"`
	AI            AI            `yaml:"ai"`
	Payments      Payments      `yaml:"payments"`
	Messaging     Messaging     `yaml:"messaging"`
	Infra         Infra         `yaml:"infra"`
	Observability Observability `yaml:"observability"`
	Compliance    Compliance    `yaml:"compliance"`
}

// Frontend describes the client-side stack.
type Frontend struct {
	Framework       string `yaml:"framework"`
	UILibrary       string `yaml:"ui_library"`
	StateManagement string `yaml:"state_management"`
	Testing         string `yaml:"testing"`
}

// Backend describes the server-side stack.
type Backend struct {
	Language  string `yaml:"language"`
	Framework string `yaml:"framework"`
	APIStyle  string `yaml:"api_style"`
	Auth      string `yaml:"auth"`
	Testing   string `yaml:"testing"`
}

// Database describes persistence choices.
type Database struct {
	Primary    string `yaml:"primary"`
	Cache      string `yaml:"cache"`
	Search     string `yaml:"search"`
	Migrations string `yaml:"migrations"`
}

// AI describes AI/ML tooling choices.
type AI struct {
	Orchestration string   `yaml:"orchestration"`
	LLMProviders  []string `yaml:"llm_providers"`
	Embedding     string   `yaml:"embedding"`
	VectorStore   string   `yaml:"vector_store"`
	Observability string   `yaml:"observability"`
}

// Payments describes payment processing choices.
type Payments struct {
	Providers []string `yaml:"providers"`
	Model     string   `yaml:"model"`
}

// Messaging describes async messaging choices.
type Messaging struct {
	Queue    string `yaml:"queue"`
	Realtime string `yaml:"realtime"`
	Email    string `yaml:"email"`
	SMS      string `yaml:"sms"`
}

// Infra describes infrastructure and deployment choices.
type Infra struct {
	Cloud     string `yaml:"cloud"`
	Container string `yaml:"container"`
	CDN       string `yaml:"cdn"`
	Secrets   string `yaml:"secrets"`
	CICD      string `yaml:"ci_cd"`
}

// Observability describes monitoring and logging choices.
type Observability struct {
	Metrics  string `yaml:"metrics"`
	Tracing  string `yaml:"tracing"`
	Logging  string `yaml:"logging"`
	Alerting string `yaml:"alerting"`
}

// Compliance describes regulatory and security compliance choices.
type Compliance struct {
	Standards      []string `yaml:"standards"`
	SecretScanning string   `yaml:"secret_scanning"`
}

// Parse reads a TSD document from r and returns the parsed TSD.
// Unknown top-level and stack-level keys are collected into TSD.UnknownKeys
// (as warnings) rather than causing an error, enabling forward-compatibility.
// Returns ErrUnsupportedVersion when tsd_version != 1.
func Parse(r io.Reader) (*TSD, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("tsd: read: %w", err)
	}
	if len(data) == 0 {
		return nil, errors.New("tsd: file is empty")
	}

	// Parse into raw node first to collect unknown keys.
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("tsd: yaml parse: %w", err)
	}
	if root.Kind == 0 || len(root.Content) == 0 {
		return nil, errors.New("tsd: document is empty")
	}

	// Decode into the typed struct.
	var t TSD
	if err := root.Decode(&t); err != nil {
		return nil, fmt.Errorf("tsd: decode: %w", err)
	}

	if t.TSDVersion != 1 {
		return nil, errcode.Newf(ErrUnsupportedVersion, nil,
			"tsd_version must be 1, got %d", t.TSDVersion)
	}

	// Collect unknown keys (forward-compat warnings).
	t.UnknownKeys = collectUnknownKeys(&root)

	return &t, nil
}

// ParseFile is a convenience wrapper that opens the file at path and calls Parse.
func ParseFile(path string) (*TSD, error) {
	f, err := os.Open(path) //nolint:gosec // caller-supplied path is intentional
	if err != nil {
		return nil, fmt.Errorf("tsd: open %s: %w", path, err)
	}
	defer f.Close()
	return Parse(f)
}

// collectUnknownKeys walks the YAML node tree and returns dotted paths
// for any keys that are not part of the TSD schema at the top and stack levels.
func collectUnknownKeys(node *yaml.Node) []string {
	// Unwrap document node.
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}
	if node.Kind != yaml.MappingNode {
		return nil
	}

	var out []string

	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]

		if !knownTopKeys[key] {
			out = append(out, key)
			continue
		}

		// Walk one level into `stack` to catch unknown sub-keys.
		if key == "stack" && val.Kind == yaml.MappingNode {
			for j := 0; j+1 < len(val.Content); j += 2 {
				sk := val.Content[j].Value
				if !knownStackKeys[sk] {
					out = append(out, "stack."+sk)
				}
			}
		}
	}

	sort.Strings(out)
	return out
}
