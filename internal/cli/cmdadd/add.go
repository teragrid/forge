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

// Package cmdadd implements `forge add <primitive>` (M3-29).
//
// forge add scaffolds well-defined project primitives into an existing
// project using the same template engine as `forge generate`.
//
// Supported primitives:
//
//	middleware   â€” HTTP middleware function with test
//	repository   â€” data-access repository with interface + stub
//	service      â€” service layer with interface + stub
//	worker       â€” background worker with graceful shutdown
//	schema       â€” database schema file (SQL) with migration pair
//	config       â€” typed configuration struct + loader
package cmdadd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 5700..5799).
var (
	ErrAddFailed = errcode.Register(errcode.Code(5700), "forge add failed")
)

// validPrimitives lists primitives `forge add` knows about.
var validPrimitives = []string{
	"middleware", "repository", "service", "worker", "schema", "config",
}

// AddResult is the output of `forge add`.
type AddResult struct {
	Primitive string   `json:"primitive"`
	Name      string   `json:"name"`
	Mode      string   `json:"mode"`
	Files     []string `json:"files"`
	Created   int      `json:"created"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "add",
		Summary: "Scaffold a project primitive (middleware, repository, service, worker, schema, config) (M3).",
		Inputs: []string{
			"<primitive>  â€” one of: middleware, repository, service, worker, schema, config",
			"<name>       â€” name for the new primitive",
			"--apply      â€” write files (default: dry-run preview)",
			"--output-dir â€” override output directory",
			"--root <path>",
			"--json",
		},
		Outputs:      []string{"stdout: list of files created / preview"},
		SideEffects:  []string{"with --apply: writes new files to project"},
		GatesTouched: []string{"Â§4 scaffold"},
		ErrorCodes:   []errcode.Code{ErrAddFailed},
	})
}

// New returns the cobra command for `forge add`.
func New() *cobra.Command {
	var (
		root      string
		apply     bool
		outputDir string
		jsonOut   bool
	)
	cmd := &cobra.Command{
		Use:   "add <primitive> <name>",
		Short: "Scaffold a project primitive into an existing project (M3).",
		Long: "forge add generates well-defined project primitives:\n\n" +
			"  middleware   â€” HTTP middleware with test\n" +
			"  repository   â€” data-access repository interface + stub\n" +
			"  service      â€” service layer interface + stub\n" +
			"  worker       â€” background worker with graceful shutdown\n" +
			"  schema       â€” SQL schema + up/down migration pair\n" +
			"  config       â€” typed config struct + loader\n\n" +
			"Default is dry-run. Pass --apply to write files.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			primitive := args[0]
			name := args[1]
			if !isValidPrimitive(primitive) {
				return errcode.Newf(ErrAddFailed, nil,
					"unknown primitive %q; valid: %s", primitive, strings.Join(validPrimitives, ", "))
			}
			r, err := resolveRoot(root)
			if err != nil {
				return err
			}
			mode := "dry-run"
			if apply {
				mode = "apply"
			}
			files, err := scaffoldPrimitive(r, primitive, name, outputDir, apply)
			if err != nil {
				return errcode.New(ErrAddFailed, "scaffold primitive", err)
			}
			res := AddResult{
				Primitive: primitive,
				Name:      name,
				Mode:      mode,
				Files:     files,
				Created:   len(files),
			}
			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
			}
			verb := "would create"
			if apply {
				verb = "created"
			}
			for _, f := range files {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", verb, f)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nadd: %s %s/%s â€” %d files %s\n",
				primitive, primitive, name, len(files), verb)
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "Project root (default: cwd)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Write files (default: dry-run preview)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Override output directory")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON output")
	return cmd
}

func isValidPrimitive(p string) bool {
	for _, v := range validPrimitives {
		if v == p {
			return true
		}
	}
	return false
}

func scaffoldPrimitive(root, primitive, name, outputDirOverride string, apply bool) ([]string, error) {
	pkg := strings.ToLower(strings.ReplaceAll(name, "-", ""))
	pascal := toPascal(name)
	module := detectModule(root)
	ts := time.Now().Format("20060102150405")

	var dir string
	if outputDirOverride != "" {
		dir = filepath.Join(root, outputDirOverride)
	}

	templates := primitiveTemplates(primitive, root, pkg, pascal, name, module, ts, dir)
	var created []string
	for _, t := range templates {
		created = append(created, t.path)
		if apply {
			if err := os.MkdirAll(filepath.Dir(t.path), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(t.path, []byte(t.content), 0o600); err != nil {
				return nil, err
			}
		}
	}
	return created, nil
}

type primFile struct {
	path    string
	content string
}

func primitiveTemplates(primitive, root, pkg, pascal, name, module, ts, dirOverride string) []primFile {
	base := func(subdir string) string {
		if dirOverride != "" {
			return dirOverride
		}
		return filepath.Join(root, "internal", subdir)
	}
	switch primitive {
	case "middleware":
		dir := base("middleware")
		return []primFile{
			{filepath.Join(dir, pkg+".go"), fmt.Sprintf(
				"package middleware\n\nimport \"net/http\"\n\n// %s is an HTTP middleware that ...\nfunc %s(next http.Handler) http.Handler {\n\treturn http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n\t\t// TODO: implement %s middleware\n\t\tnext.ServeHTTP(w, r)\n\t})\n}\n",
				pascal, pascal, name)},
			{filepath.Join(dir, pkg+"_test.go"), fmt.Sprintf(
				"package middleware_test\n\nimport (\n\t\"net/http\"\n\t\"net/http/httptest\"\n\t\"testing\"\n\n\t\"%s/internal/middleware\"\n)\n\nfunc Test%s(t *testing.T) {\n\thandler := middleware.%s(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {\n\t\tw.WriteHeader(http.StatusOK)\n\t}))\n\treq := httptest.NewRequest(http.MethodGet, \"/\", nil)\n\tw := httptest.NewRecorder()\n\thandler.ServeHTTP(w, req)\n\tif w.Code != http.StatusOK {\n\t\tt.Errorf(\"expected 200 got %%d\", w.Code)\n\t}\n}\n",
				module, pascal, pascal)},
		}
	case "repository":
		dir := base(pkg)
		return []primFile{
			{filepath.Join(dir, "repository.go"), fmt.Sprintf(
				"package %s\n\nimport \"context\"\n\n// %sRepository defines the data-access contract for %s.\ntype %sRepository interface {\n\tFindByID(ctx context.Context, id string) (*%s, error)\n\tSave(ctx context.Context, e *%s) error\n\tDelete(ctx context.Context, id string) error\n}\n\n// %s is the domain entity.\ntype %s struct {\n\tID string\n}\n",
				pkg, pascal, name, pascal, pascal, pascal, pascal, pascal)},
			{filepath.Join(dir, "memory_repository.go"), fmt.Sprintf(
				"package %s\n\nimport (\n\t\"context\"\n\t\"fmt\"\n\t\"sync\"\n)\n\n// Memory%sRepository is an in-memory implementation of %sRepository.\ntype Memory%sRepository struct {\n\tmu   sync.RWMutex\n\tdata map[string]*%s\n}\n\nfunc NewMemory%sRepository() *Memory%sRepository {\n\treturn &Memory%sRepository{data: map[string]*%s{}}\n}\n\nfunc (r *Memory%sRepository) FindByID(_ context.Context, id string) (*%s, error) {\n\tr.mu.RLock()\n\tdefer r.mu.RUnlock()\n\te, ok := r.data[id]\n\tif !ok {\n\t\treturn nil, fmt.Errorf(\"%s %%q not found\", id)\n\t}\n\treturn e, nil\n}\n\nfunc (r *Memory%sRepository) Save(_ context.Context, e *%s) error {\n\tr.mu.Lock()\n\tdefer r.mu.Unlock()\n\tr.data[e.ID] = e\n\treturn nil\n}\n\nfunc (r *Memory%sRepository) Delete(_ context.Context, id string) error {\n\tr.mu.Lock()\n\tdefer r.mu.Unlock()\n\tdelete(r.data, id)\n\treturn nil\n}\n",
				pkg, pascal, pascal, pascal, pascal, pascal, pascal, pascal, pascal, pascal, pascal, name, pascal, pascal, pascal)},
		}
	case "service":
		dir := base(pkg)
		return []primFile{
			{filepath.Join(dir, "service.go"), fmt.Sprintf(
				"package %s\n\nimport \"context\"\n\n// %sService defines the business-logic contract for %s.\ntype %sService interface {\n\tCreate(ctx context.Context, req Create%sRequest) (*%s, error)\n\tGet(ctx context.Context, id string) (*%s, error)\n}\n\n// Create%sRequest is the input for creating a %s.\ntype Create%sRequest struct {\n\t// TODO: add fields\n}\n",
				pkg, pascal, name, pascal, pascal, pascal, pascal, pascal, name, pascal)},
		}
	case "worker":
		dir := base(pkg)
		return []primFile{
			{filepath.Join(dir, "worker.go"), fmt.Sprintf(
				"package %s\n\nimport (\n\t\"context\"\n\t\"log/slog\"\n)\n\n// %sWorker is a background worker for %s.\ntype %sWorker struct {\n\tlog *slog.Logger\n}\n\nfunc New%sWorker(log *slog.Logger) *%sWorker {\n\treturn &%sWorker{log: log}\n}\n\n// Run starts the worker and blocks until ctx is cancelled.\nfunc (w *%sWorker) Run(ctx context.Context) error {\n\tw.log.Info(\"worker started\", \"name\", \"%s\")\n\t<-ctx.Done()\n\tw.log.Info(\"worker stopping\", \"name\", \"%s\")\n\treturn nil\n}\n",
				pkg, pascal, name, pascal, pascal, pascal, pascal, pascal, name, name)},
		}
	case "schema":
		dir := base("migrations")
		if dirOverride != "" {
			dir = dirOverride
		}
		return []primFile{
			{filepath.Join(dir, ts+"_"+pkg+".up.sql"), fmt.Sprintf(
				"-- Migration: %s (up)\n-- Created: %s\n\nCREATE TABLE IF NOT EXISTS %ss (\n\tid   TEXT PRIMARY KEY,\n\t-- TODO: add columns\n\tcreated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()\n);\n", name, ts, pkg)},
			{filepath.Join(dir, ts+"_"+pkg+".down.sql"), fmt.Sprintf(
				"-- Migration: %s (down)\n-- Created: %s\n\nDROP TABLE IF EXISTS %ss;\n", name, ts, pkg)},
		}
	case "config":
		dir := base("config")
		return []primFile{
			{filepath.Join(dir, pkg+".go"), fmt.Sprintf(
				"package config\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\n// %sConfig holds configuration for the %s component.\ntype %sConfig struct {\n\t// TODO: add fields\n}\n\n// Load%sConfig reads configuration from environment variables.\nfunc Load%sConfig() (%sConfig, error) {\n\tvar cfg %sConfig\n\t// TODO: populate from env\n\t_ = os.Getenv(\"APP_%s\")\n\treturn cfg, validate%sConfig(cfg)\n}\n\nfunc validate%sConfig(cfg %sConfig) error {\n\t_ = cfg\n\t// TODO: validate required fields\n\treturn fmt.Errorf(\"validate%sConfig: not yet implemented\")\n}\n",
				pascal, name, pascal, pascal, pascal, pascal, pascal,
				strings.ToUpper(pkg), pascal, pascal, pascal, pascal)},
		}
	}
	return nil
}

func toPascal(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	var b strings.Builder
	for _, p := range parts {
		if len(p) > 0 {
			b.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return b.String()
}

func detectModule(root string) string {
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

func resolveRoot(root string) (string, error) {
	if root != "" {
		return root, nil
	}
	return os.Getwd()
}
