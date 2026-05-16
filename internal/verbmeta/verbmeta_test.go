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
package verbmeta

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRegisterAndLookup(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	Register(Manifest{Verb: "demo", Summary: "demo verb"})
	m, ok := Lookup("demo")
	if !ok {
		t.Fatal("Lookup missed registered verb")
	}
	if m.Inputs == nil || m.Outputs == nil || m.SideEffects == nil {
		t.Fatal("nil slices must be normalised to empty for stable JSON")
	}
}

func TestRegister_DuplicatePanics(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	Register(Manifest{Verb: "dup"})
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate verb")
		}
	}()
	Register(Manifest{Verb: "dup"})
}

func TestRegister_EmptyVerbPanics(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on empty verb")
		}
	}()
	Register(Manifest{Verb: ""})
}

func TestAll_Sorted(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	Register(Manifest{Verb: "zeta"})
	Register(Manifest{Verb: "alpha"})
	Register(Manifest{Verb: "mu"})
	out := All()
	if len(out) != 3 || out[0].Verb != "alpha" || out[1].Verb != "mu" || out[2].Verb != "zeta" {
		t.Fatalf("All() not sorted: %+v", out)
	}
}

// ── DEV-M0-11 JSON schema validation ─────────────────────────────────────────

// TC-11-01 (happy): ValidateJSON passes when all OutputFields are present.
func TestValidateJSON_AllFieldsPresent(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	m := Manifest{
		Verb:         "testverb",
		OutputFields: []string{"status", "count"},
	}
	json := []byte(`{"status":"clean","count":0,"extra":"ignored"}`)
	if err := m.ValidateJSON(json); err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

// TC-11-02 (negative): ValidateJSON fails when a required field is missing.
func TestValidateJSON_MissingField(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	m := Manifest{
		Verb:         "testverb",
		OutputFields: []string{"status", "count", "required_field"},
	}
	json := []byte(`{"status":"clean","count":0}`)
	if err := m.ValidateJSON(json); err == nil {
		t.Error("expected error for missing required_field")
	}
}

// TC-11-03 (boundary): ValidateJSON with no OutputFields always passes.
func TestValidateJSON_NoOutputFields(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	m := Manifest{Verb: "noschema"}
	if err := m.ValidateJSON([]byte(`{}`)); err != nil {
		t.Errorf("expected nil for no OutputFields, got: %v", err)
	}
}

// TC-11-04 (false-positive guard): ValidateJSON ignores non-object JSON (arrays).
func TestValidateJSON_ArrayJSON_Ignored(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	m := Manifest{
		Verb:         "arr",
		OutputFields: []string{"status"},
	}
	// Array output → not an object → skip schema check
	if err := m.ValidateJSON([]byte(`[{"status":"ok"}]`)); err != nil {
		t.Errorf("expected nil for array JSON, got: %v", err)
	}
}

// TC-11-05 (idempotency): calling ValidateJSON twice on valid output is stable.
func TestValidateJSON_Idempotent(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	m := Manifest{
		Verb:         "idem",
		OutputFields: []string{"findings"},
	}
	data := []byte(`{"findings":[],"count":0}`)
	if err := m.ValidateJSON(data); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := m.ValidateJSON(data); err != nil {
		t.Fatalf("second call: %v", err)
	}
}

// ── G-082: GenerateCLISchemas ─────────────────────────────────────────────────

// TestGenerateCLISchemas_CreatesFiles verifies that GenerateCLISchemas writes
// one .schema.json file per registered verb into .forge/cli-schemas/.
func TestGenerateCLISchemas_CreatesFiles(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	Register(Manifest{Verb: "alpha", Summary: "alpha verb", OutputFields: []string{"status", "count"}})
	Register(Manifest{Verb: "beta", Summary: "beta verb"})

	dir := t.TempDir()
	if err := GenerateCLISchemas(dir); err != nil {
		t.Fatalf("GenerateCLISchemas: %v", err)
	}

	for _, verb := range []string{"alpha", "beta"} {
		path := filepath.Join(dir, ".forge", "cli-schemas", verb+".schema.json")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("schema file missing for verb %q: %v", verb, err)
		}
	}
}

// TestGenerateCLISchemas_SchemaContents verifies the generated JSON schema has
// the expected fields for a verb with declared OutputFields.
func TestGenerateCLISchemas_SchemaContents(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	Register(Manifest{
		Verb:         "myscan",
		Summary:      "scanner verb",
		OutputFields: []string{"findings", "count", "status"},
	})

	dir := t.TempDir()
	if err := GenerateCLISchemas(dir); err != nil {
		t.Fatalf("GenerateCLISchemas: %v", err)
	}

	path := filepath.Join(dir, ".forge", "cli-schemas", "myscan.schema.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	var schema CLISchema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("expected type=object, got %q", schema.Type)
	}
	for _, f := range []string{"findings", "count", "status"} {
		if _, ok := schema.Properties[f]; !ok {
			t.Errorf("property %q missing from schema", f)
		}
	}
	if len(schema.Required) != 3 {
		t.Errorf("expected 3 required fields, got %d", len(schema.Required))
	}
}

// TestGenerateCLISchemas_NoOutputFields verifies that verbs with no OutputFields
// get a schema with no required list (not nil-crash).
func TestGenerateCLISchemas_NoOutputFields(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	Register(Manifest{Verb: "empty", Summary: "no outputs"})

	dir := t.TempDir()
	if err := GenerateCLISchemas(dir); err != nil {
		t.Fatalf("GenerateCLISchemas: %v", err)
	}

	path := filepath.Join(dir, ".forge", "cli-schemas", "empty.schema.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schema CLISchema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if schema.Required != nil {
		t.Errorf("expected nil Required for verb with no OutputFields, got %v", schema.Required)
	}
}

// TestGenerateCLISchemas_Idempotent verifies that calling GenerateCLISchemas
// twice overwrites cleanly without error.
func TestGenerateCLISchemas_Idempotent(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	Register(Manifest{Verb: "idem2", OutputFields: []string{"status"}})

	dir := t.TempDir()
	if err := GenerateCLISchemas(dir); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := GenerateCLISchemas(dir); err != nil {
		t.Fatalf("second run: %v", err)
	}
}
