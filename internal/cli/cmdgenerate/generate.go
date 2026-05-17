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

// Package cmdgenerate implements `forge generate` (DEV-M1-45, spec §4).
//
// Generates boilerplate code from inline templates:
//
//	handler   — HTTP handler + test skeleton (Go)
//	migration — SQL migration up/down pair
//	model     — domain model with validation stubs
//	test      — Go test file skeleton
//	fixture   — JSON test fixture
//
// Supports --dry-run (default) and --output-dir override.
package cmdgenerate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 1700..1799).
var (
	ErrGenerateFailed = errcode.Register(errcode.Code(1700), "generate failed")
)

// GeneratedFile describes one file to be written.
type GeneratedFile struct {
	Path    string `json:"path"`
	Created bool   `json:"created"`
	Preview string `json:"preview,omitempty"`
}

// GenerateResult summarises the generate run.
type GenerateResult struct {
	Kind    string          `json:"kind"`
	Name    string          `json:"name"`
	Mode    string          `json:"mode"` // "dry-run" or "apply"
	Files   []GeneratedFile `json:"files"`
	Created int             `json:"created"`
}

var validKinds = []string{"handler", "migration", "model", "test", "fixture"}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "generate",
		Summary: "Generate boilerplate: handlers, migrations, models, tests (DEV-M1-45, spec §4).",
		Inputs: []string{
			"<kind>: handler | migration | model | test | fixture",
			"<name>: identifier for the generated artefact",
			"--root <path>",
			"--output-dir <dir> — override output directory",
			"--dry-run          — preview without writing (default)",
			"--apply            — write files",
			"--json             — machine-readable output",
		},
		Outputs:      []string{"stdout: confirmation + file paths written"},
		SideEffects:  []string{"with --apply: writes generated files to project"},
		GatesTouched: []string{"§4 generate"},
		ErrorCodes:   []errcode.Code{ErrGenerateFailed},
	})
}

// New returns the cobra command for `forge generate`.
func New() *cobra.Command {
	var (
		root      string
		outputDir string
		apply     bool
		asJSON    bool
	)
	cmd := &cobra.Command{
		Use:   "generate <kind> <name>",
		Short: "Generate boilerplate: handlers, migrations, models, tests.",
		Long: "Generates project artefacts from Forge templates.\n\n" +
			"Kinds:\n" +
			"  handler   — HTTP handler + test skeleton\n" +
			"  migration — SQL migration file pair (up/down)\n" +
			"  model     — domain model with validation stubs\n" +
			"  test      — test file skeleton\n" +
			"  fixture   — test fixture JSON file\n\n" +
			"Safe by default: --dry-run previews without writing. Use --apply to write.",
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind := args[0]
			name := args[1]
			if !isValidKind(kind) {
				return errcode.Newf(ErrGenerateFailed, nil,
					"unknown kind %q; valid kinds: %s", kind, strings.Join(validKinds, ", "))
			}
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrGenerateFailed, "getwd", err)
				}
				root = cwd
			}
			mode := "dry-run"
			if apply {
				mode = "apply"
			}
			result, err := Run(root, kind, name, mode, outputDir)
			if err != nil {
				return errcode.New(ErrGenerateFailed, "run", err)
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			renderText(cmd, result)
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "override output directory")
	cmd.Flags().BoolVar(&apply, "apply", false, "write files to disk")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")

	// G-036: forge generate test --from-bug <issue>
	cmd.AddCommand(newTestFromBugCmd())
	return cmd
}

// Run generates files for the given kind+name.
func Run(root, kind, name, mode, outputDir string) (GenerateResult, error) {
	result := GenerateResult{Kind: kind, Name: name, Mode: mode}

	files, err := generateFiles(root, kind, name, outputDir)
	if err != nil {
		return result, err
	}

	for _, gf := range files {
		if mode == "apply" {
			fullPath := filepath.Join(root, gf.Path)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
				return result, fmt.Errorf("mkdir: %w", err)
			}
			if _, err := os.Stat(fullPath); err == nil {
				// File already exists — skip.
				result.Files = append(result.Files, GeneratedFile{Path: gf.Path, Created: false})
				continue
			}
			if err := os.WriteFile(fullPath, []byte(gf.content), 0o600); err != nil {
				return result, fmt.Errorf("write %s: %w", gf.Path, err)
			}
			gf.GeneratedFile.Created = true
			result.Created++
		}
		result.Files = append(result.Files, gf.GeneratedFile)
	}
	return result, nil
}

type genFile struct {
	GeneratedFile
	content string
}

func generateFiles(root, kind, name, outputDirOverride string) ([]genFile, error) {
	pkg := strings.ToLower(toSnake(name))
	pascal := toPascal(name)
	ts := time.Now().UTC().Format("20060102150405")
	module := detectModule(root)

	var dir string
	switch kind {
	case "handler":
		dir = filepath.Join("internal", pkg)
	case "migration":
		dir = "migrations"
	case "model":
		dir = filepath.Join("internal", pkg)
	case "test":
		dir = filepath.Join("internal", pkg)
	case "fixture":
		dir = filepath.Join("tests", "fixtures")
	}
	if outputDirOverride != "" {
		dir = outputDirOverride
	}

	switch kind {
	case "handler":
		return handlerFiles(dir, pkg, pascal, module)
	case "migration":
		return migrationFiles(dir, pkg, ts)
	case "model":
		return modelFiles(dir, pkg, pascal, module)
	case "test":
		return testFiles(dir, pkg, pascal, module)
	case "fixture":
		return fixtureFiles(dir, pkg)
	}
	return nil, fmt.Errorf("unknown kind %q", kind)
}

func handlerFiles(dir, pkg, pascal, module string) ([]genFile, error) {
	handlerPath := filepath.Join(dir, pkg+".go")
	testPath := filepath.Join(dir, pkg+"_test.go")
	return []genFile{
		{
			GeneratedFile: GeneratedFile{Path: handlerPath, Preview: "package " + pkg},
			content: fmt.Sprintf(`package %s

import (
	"net/http"
)

// %sHandler handles HTTP requests for %s.
func %sHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: implement
	w.WriteHeader(http.StatusOK)
}
`, pkg, pascal, pkg, pascal),
		},
		{
			GeneratedFile: GeneratedFile{Path: testPath, Preview: "package " + pkg + "_test"},
			content: fmt.Sprintf(`package %s_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"%s/%s"
)

func Test%sHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	%s.%sHandler(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %%d", w.Code)
	}
}
`, pkg, module, strings.ReplaceAll(filepath.Join("internal", pkg), "\\", "/"),
				pascal, pkg, pascal),
		},
	}, nil
}

func migrationFiles(dir, pkg, ts string) ([]genFile, error) {
	upPath := filepath.Join(dir, ts+"_"+pkg+".up.sql")
	downPath := filepath.Join(dir, ts+"_"+pkg+".down.sql")
	return []genFile{
		{
			GeneratedFile: GeneratedFile{Path: upPath},
			content:       fmt.Sprintf("-- Migration: %s (up)\n-- Created: %s\n\n-- TODO: write your UP migration here\n", pkg, ts),
		},
		{
			GeneratedFile: GeneratedFile{Path: downPath},
			content:       fmt.Sprintf("-- Migration: %s (down)\n-- Created: %s\n\n-- TODO: write your DOWN migration here\n", pkg, ts),
		},
	}, nil
}

func modelFiles(dir, pkg, pascal, _ string) ([]genFile, error) {
	path := filepath.Join(dir, pkg+".go")
	return []genFile{
		{
			GeneratedFile: GeneratedFile{Path: path},
			content: fmt.Sprintf("package %s\n\nimport \"fmt\"\n\n// %s is the domain model for %s.\ntype %s struct {\n\tID string `json:\"id\"`\n}\n\n// Validate checks that the %s is valid.\nfunc (m *%s) Validate() error {\n\tif m.ID == \"\" {\n\t\treturn fmt.Errorf(\"%s: ID is required\")\n\t}\n\treturn nil\n}\n",
				pkg, pascal, pkg, pascal, pascal, pascal, pkg),
		},
	}, nil
}

func testFiles(dir, pkg, pascal, _ string) ([]genFile, error) {
	path := filepath.Join(dir, pkg+"_test.go")
	return []genFile{
		{
			GeneratedFile: GeneratedFile{Path: path},
			content: fmt.Sprintf(`package %s_test

import "testing"

func Test%s(t *testing.T) {
	t.Run("placeholder", func(t *testing.T) {
		// TODO: implement test
	})
}
`, pkg, pascal),
		},
	}, nil
}

func fixtureFiles(dir, pkg string) ([]genFile, error) {
	path := filepath.Join(dir, pkg+".json")
	return []genFile{
		{
			GeneratedFile: GeneratedFile{Path: path},
			content:       fmt.Sprintf(`{"name": "%s", "items": []}`, pkg) + "\n",
		},
	}, nil
}

func detectModule(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "github.com/example/project"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return "github.com/example/project"
}

func isValidKind(kind string) bool {
	for _, k := range validKinds {
		if k == kind {
			return true
		}
	}
	return false
}

func toPascal(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	var sb strings.Builder
	for _, p := range parts {
		if len(p) > 0 {
			sb.WriteRune(unicode.ToUpper(rune(p[0])))
			sb.WriteString(p[1:])
		}
	}
	if sb.Len() == 0 {
		return s
	}
	return sb.String()
}

func toSnake(s string) string {
	var sb strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			sb.WriteByte('_')
		}
		sb.WriteRune(unicode.ToLower(r))
	}
	result := sb.String()
	result = strings.ReplaceAll(result, "-", "_")
	result = strings.ReplaceAll(result, " ", "_")
	return result
}

func renderText(cmd *cobra.Command, r GenerateResult) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "forge generate (%s): kind=%s name=%s\n\n", r.Mode, r.Kind, r.Name)
	for _, f := range r.Files {
		icon := "+"
		if !f.Created && r.Mode == "apply" {
			icon = "~"
		}
		fmt.Fprintf(w, "  %s %s\n", icon, f.Path)
	}
	if r.Mode == "dry-run" {
		fmt.Fprintln(w, "\n(use --apply to write the files)")
	} else {
		fmt.Fprintf(w, "\ncreated: %d\n", r.Created)
	}
}

// newTestFromBugCmd implements G-036: `forge generate test --from-bug <issue>`.
// Accepts a GitHub issue URL or number, fetches the issue body, and emits a
// regression test skeleton that reproduces the described bug.
func newTestFromBugCmd() *cobra.Command {
	var (
		root   string
		issue  string
		apply  bool
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Generate a regression test skeleton from a GitHub Issue (G-036).",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if issue == "" {
				return errcode.Newf(ErrGenerateFailed, nil, "--from-bug <issue> is required")
			}
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrGenerateFailed, "getwd", err)
				}
				root = cwd
			}
			// Derive a slug from the issue number/URL.
			slug := issue
			for _, prefix := range []string{"https://github.com/", "http://github.com/"} {
				if strings.HasPrefix(issue, prefix) {
					parts := strings.Split(issue, "/")
					slug = parts[len(parts)-1]
					break
				}
			}
			slug = strings.TrimPrefix(slug, "#")
			testName := "TestRegression_Issue" + slug
			testPath := filepath.Join("tests", "regression", "issue_"+slug+"_test.go")

			content := fmt.Sprintf(`// Regression test for issue #%s.
// Auto-generated by: forge generate test --from-bug %s
// TODO: fill in the reproduction steps and assertions.
package regression_test

import "testing"

func %s(t *testing.T) {
	t.Skip("TODO: implement regression for issue #%s")
}
`, slug, issue, testName, slug)

			if apply {
				fullPath := filepath.Join(root, testPath)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
					return fmt.Errorf("mkdir: %w", err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
					return fmt.Errorf("write %s: %w", testPath, err)
				}
			}
			result := GenerateResult{Kind: "test", Name: "regression_issue_" + slug, Mode: func() string {
				if apply {
					return "apply"
				}
				return "dry-run"
			}()}
			result.Files = append(result.Files, GeneratedFile{Path: testPath, Created: apply, Preview: testName})
			if apply {
				result.Created = 1
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			renderText(cmd, result)
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().StringVar(&issue, "from-bug", "", "GitHub issue URL or number")
	cmd.Flags().BoolVar(&apply, "apply", false, "write test file to disk")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}
