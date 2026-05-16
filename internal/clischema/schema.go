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

// Package clischema implements G-082: per-command JSON schema generation.
//
// GenerateSchema() produces a JSON Schema v7 document for a cobra.Command's
// flags and emits it to .forge/cli-schemas/<command>.schema.json. This allows
// external tools (editors, CI gates) to validate `--json` output against the
// published schema.
package clischema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Schema is a minimal JSON Schema v7 document for a CLI command.
type Schema struct {
	Schema      string              `json:"$schema"`
	Title       string              `json:"title"`
	Description string              `json:"description,omitempty"`
	Type        string              `json:"type"`
	Properties  map[string]Property `json:"properties,omitempty"`
	Required    []string            `json:"required,omitempty"`
}

// Property is one field in the JSON schema.
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
}

// GenerateSchema builds a JSON Schema for cmd's flags and writes it to
// <root>/.forge/cli-schemas/<commandName>.schema.json.
func GenerateSchema(root string, cmd *cobra.Command) error {
	schema := buildSchema(cmd)
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal schema for %s: %w", cmd.Name(), err)
	}
	dir := filepath.Join(root, ".forge", "cli-schemas")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir cli-schemas: %w", err)
	}
	outPath := filepath.Join(dir, cmd.Name()+".schema.json")
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("write schema %s: %w", outPath, err)
	}
	return nil
}

// GenerateAll generates schemas for all commands in the tree rooted at root.
func GenerateAll(projectRoot string, root *cobra.Command) error {
	var errs []string
	for _, cmd := range root.Commands() {
		if err := GenerateSchema(projectRoot, cmd); err != nil {
			errs = append(errs, err.Error())
		}
		// Recurse into sub-commands.
		for _, sub := range cmd.Commands() {
			if err := GenerateSchema(projectRoot, sub); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("schema generation errors:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

func buildSchema(cmd *cobra.Command) Schema {
	props := map[string]Property{}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		props[f.Name] = Property{
			Type:        flagType(f),
			Description: f.Usage,
			Default:     f.DefValue,
		}
	})
	return Schema{
		Schema:      "http://json-schema.org/draft-07/schema#",
		Title:       "forge " + cmd.CommandPath(),
		Description: cmd.Short,
		Type:        "object",
		Properties:  props,
	}
}

func flagType(f *pflag.Flag) string {
	switch f.Value.Type() {
	case "bool":
		return "boolean"
	case "int", "int64", "uint", "uint64":
		return "integer"
	case "float64":
		return "number"
	default:
		return "string"
	}
}
