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

import "testing"

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
