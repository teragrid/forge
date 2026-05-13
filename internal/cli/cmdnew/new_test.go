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
package cmdnew

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := New("9.9.9-test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestNew_Happy(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "demo")
	out, err := runCmd(t, "go-service", target)
	if err != nil {
		t.Fatalf("run: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "scaffolded") {
		t.Fatalf("missing scaffold confirm: %s", out)
	}
	if _, err := os.Stat(filepath.Join(target, "main.go")); err != nil {
		t.Fatalf("main.go missing: %v", err)
	}
}

func TestNew_JSONOutput(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "demo")
	out, err := runCmd(t, "go-service", target, "--json")
	if err != nil {
		t.Fatalf("run: %v\nout: %s", err, out)
	}
	var res struct {
		Template string   `json:"Template"`
		Files    []string `json:"Files"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("not JSON: %v: %s", err, out)
	}
	if res.Template != "go-service" || len(res.Files) == 0 {
		t.Fatalf("bad json: %+v", res)
	}
}

func TestNew_UnknownTemplate(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "demo")
	out, err := runCmd(t, "no-such-template", target)
	if err == nil {
		t.Fatalf("expected error, got: %s", out)
	}
	if !strings.Contains(err.Error(), "FORGE-2200") {
		t.Fatalf("want FORGE-2200, got %v", err)
	}
}

func TestNew_NeedsBothArgs(t *testing.T) {
	t.Parallel()
	if _, err := runCmd(t, "go-service"); err == nil {
		t.Fatal("expected arg-count error")
	}
}

func TestNew_List_Text(t *testing.T) {
	t.Parallel()
	out, err := runCmd(t, "--list")
	if err != nil {
		t.Fatalf("--list failed: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "go-service") {
		t.Fatalf("go-service missing from list: %s", out)
	}
	if !strings.Contains(out, "ts-service") {
		t.Fatalf("ts-service missing from list: %s", out)
	}
}

func TestNew_List_JSON(t *testing.T) {
	t.Parallel()
	out, err := runCmd(t, "--list", "--json")
	if err != nil {
		t.Fatalf("--list --json failed: %v\nout: %s", err, out)
	}
	var res struct {
		Templates []string `json:"templates"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("not valid JSON: %v: %s", err, out)
	}
	if len(res.Templates) == 0 {
		t.Fatal("expected at least one template in JSON output")
	}
	found := false
	for _, tmpl := range res.Templates {
		if tmpl == "go-service" {
			found = true
		}
	}
	if !found {
		t.Fatalf("go-service missing from JSON list: %v", res.Templates)
	}
}

// TestNew_List_NoArgsRequired verifies --list works without <template> or <path>.
func TestNew_List_NoArgsRequired(t *testing.T) {
	t.Parallel()
	// Passing --list with no positional args must not error.
	if _, err := runCmd(t, "--list"); err != nil {
		t.Fatalf("--list with no args should succeed, got: %v", err)
	}
}

// TestNew_List_FalsePositive verifies that --list does NOT create any files.
func TestNew_List_FalsePositive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := runCmd(t, "--list"); err != nil {
		t.Fatalf("--list failed: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("--list must not write files; found %d entries", len(entries))
	}
}
