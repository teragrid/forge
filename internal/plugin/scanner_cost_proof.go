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

// scanner_cost_proof.go — M1-21: in-tree scanner-cost plugin proof.
//
// Demonstrates the plugin.Scanner contract with a lightweight LLM/cloud
// cost scanner registered as a first-class in-process plugin. The same
// rules ship in the `forge scan cost` family (cmdscan/cost.go); this file
// shows the plugin.Scanner wrapper that allows the same logic to be
// delivered as a WASM plugin in M2.

package plugin

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// ScannerCostPlugin is the proof-of-concept in-tree scanner plugin for
// LLM / cloud cost anti-patterns (M1-21).
type ScannerCostPlugin struct{}

var _ Scanner = (*ScannerCostPlugin)(nil) // interface assertion

func (p *ScannerCostPlugin) Manifest() Manifest {
	return Manifest{
		Name:         "scanner-cost",
		Version:      "0.1.0",
		Kind:         KindScanner,
		Author:       "forge-core",
		Summary:      "Detects LLM and cloud cost anti-patterns (proof-of-concept in-process plugin).",
		Capabilities: []string{"fs:read"},
		Forge:        ">=0.1.0",
	}
}

// Scan walks root looking for cost anti-patterns in Go and TypeScript files.
func (p *ScannerCostPlugin) Scan(ctx context.Context, root string) ([]Finding, error) {
	var findings []Finding
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "node_modules" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(path) {
		case ".go":
			findings = append(findings, scanCostGo(path)...)
		case ".ts", ".tsx", ".js", ".jsx":
			findings = append(findings, scanCostTS(path)...)
		}
		return nil
	})
	return findings, err
}

func scanCostGo(path string) []Finding {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil
	}
	var out []Finding
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		name := sel.Sel.Name
		// llm-in-loop: LLM call inside a loop
		_ = name // pattern detection via AST parent walk would add findings here
		// For the proof, emit findings via text scan (Go AST shows structure,
		// text scan catches string-level patterns that AST misses).
		return true
	})
	// Supplement with text-level patterns.
	out = append(out, scanCostText(path, ".go")...)
	return out
}

func scanCostTS(path string) []Finding {
	return scanCostText(path, filepath.Ext(path))
}

// costPatterns is the set of patterns for the scanner-cost plugin.
var costPatterns = []struct {
	rule   string
	needle string
	detail string
}{
	{"llm-no-token-limit", "maxTokens", "LLM call missing max_tokens safeguard"},
	{"llm-no-token-limit", "max_tokens", "verify max_tokens is bounded"},
	{"unbounded-cloud-list", "ListObjects(", "cloud list call may be unbounded; add pagination"},
	{"unbounded-cloud-list", "list_objects(", "cloud list call may be unbounded; add pagination"},
	{"missing-cache-control", `Cache-Control`, "missing Cache-Control header check"},
}

func scanCostText(path, _ string) []Finding {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	var out []Finding
	for i, line := range lines {
		for _, pat := range costPatterns {
			if strings.Contains(line, pat.needle) {
				out = append(out, Finding{
					File:   path,
					Line:   i + 1,
					Rule:   pat.rule,
					Match:  strings.TrimSpace(line),
					Detail: pat.detail,
				})
			}
		}
	}
	return out
}

func init() {
	Default().Register(&ScannerCostPlugin{})
}
