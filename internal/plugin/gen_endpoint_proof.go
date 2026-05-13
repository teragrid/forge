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

// gen_endpoint_proof.go â€” M1-22: in-tree generator-plugin proof.
//
// Demonstrates the plugin.Generator contract with an HTTP endpoint
// scaffold generator registered as an in-process plugin. In M2 this
// would be compiled to WASM and loaded via the wazero runtime.

package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Generator is the interface exposed by code-generation plugins.
// A Generator receives a name and options and writes files under root.
type Generator interface {
	Plugin
	Generate(ctx context.Context, root, name string, opts map[string]string) (GenerateResult, error)
}

// GenerateResult describes what the generator produced.
type GenerateResult struct {
	Files   []string `json:"files"`
	Created int      `json:"created"`
	DryRun  bool     `json:"dry_run"`
	Detail  string   `json:"detail,omitempty"`
}

// GenEndpointPlugin generates an HTTP handler + test pair for a named endpoint.
// It is the proof-of-concept in-tree generator plugin (M1-22).
type GenEndpointPlugin struct{}

var _ Generator = (*GenEndpointPlugin)(nil) // interface assertion

func (g *GenEndpointPlugin) Manifest() Manifest {
	return Manifest{
		Name:         "gen-endpoint",
		Version:      "0.1.0",
		Kind:         KindTemplate,
		Author:       "forge-core",
		Summary:      "Generates an HTTP handler + test file for a named endpoint (proof-of-concept).",
		Capabilities: []string{"fs:write"},
		Forge:        ">=0.1.0",
	}
}

// Generate writes handler.go and handler_test.go into <root>/internal/<name>/.
// Pass opts["dry_run"]="true" to skip writes.
func (g *GenEndpointPlugin) Generate(_ context.Context, root, name string, opts map[string]string) (GenerateResult, error) {
	dryRun := opts["dry_run"] == "true"
	pkg := strings.ToLower(name)
	pascal := toPascalCase(name)
	dir := filepath.Join(root, "internal", pkg)

	handlerTmpl := `package {{.Pkg}}

import "net/http"

// {{.Pascal}}Handler handles requests for the {{.Name}} endpoint.
func {{.Pascal}}Handler(w http.ResponseWriter, r *http.Request) {
	// TODO: implement {{.Name}} endpoint logic
	w.WriteHeader(http.StatusOK)
}
`
	testTmpl := `package {{.Pkg}}_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"{{.Module}}/internal/{{.Pkg}}"
)

func Test{{.Pascal}}Handler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	{{.Pkg}}.{{.Pascal}}Handler(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
`
	data := map[string]string{
		"Pkg":    pkg,
		"Pascal": pascal,
		"Name":   name,
		"Module": detectModuleName(root),
	}

	files := []struct {
		path    string
		content string
	}{
		{filepath.Join(dir, pkg+".go"), handlerTmpl},
		{filepath.Join(dir, pkg+"_test.go"), testTmpl},
	}

	var created []string
	for _, f := range files {
		content, err := renderTemplate(f.content, data)
		if err != nil {
			return GenerateResult{}, fmt.Errorf("gen-endpoint: render %s: %w", f.path, err)
		}
		if !dryRun {
			if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
				return GenerateResult{}, err
			}
			if err := os.WriteFile(f.path, []byte(content), 0o600); err != nil {
				return GenerateResult{}, err
			}
		}
		created = append(created, f.path)
	}

	return GenerateResult{
		Files:   created,
		Created: len(created),
		DryRun:  dryRun,
		Detail:  fmt.Sprintf("generated endpoint scaffold for %q in %s", name, dir),
	}, nil
}

func toPascalCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	var sb strings.Builder
	for _, p := range parts {
		if len(p) > 0 {
			sb.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return sb.String()
}

func renderTemplate(tmplStr string, data map[string]string) (string, error) {
	tmpl, err := template.New("").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func detectModuleName(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "github.com/example/app"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return "github.com/example/app"
}

func init() {
	Default().Register(&GenEndpointPlugin{})
}
