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
package cmdtelemetry

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/telemetry"
)

func exec(t *testing.T, args []string) string {
	t.Helper()
	var buf bytes.Buffer
	c := New()
	c.SetOut(&buf)
	c.SetErr(&buf)
	c.SetArgs(args)
	if err := c.Execute(); err != nil {
		t.Fatalf("exec %v: %v\n%s", args, err, buf.String())
	}
	return buf.String()
}

// TC-CTEL-01: status on fresh dir shows disabled.
func TestTelemetry_Status_Default(t *testing.T) {
	dir := t.TempDir()
	out := exec(t, []string{"status", "--root", dir})
	if !strings.Contains(out, "disabled") {
		t.Fatalf("want 'disabled' in output, got: %s", out)
	}
}

// TC-CTEL-02: enable → status shows enabled.
func TestTelemetry_Enable(t *testing.T) {
	dir := t.TempDir()
	out := exec(t, []string{"enable", "--root", dir})
	if !strings.Contains(out, "enabled") {
		t.Fatalf("want 'enabled' in output, got: %s", out)
	}
}

// TC-CTEL-03: disable after enable → status shows disabled.
func TestTelemetry_Disable(t *testing.T) {
	dir := t.TempDir()
	exec(t, []string{"enable", "--root", dir})
	out := exec(t, []string{"disable", "--root", dir})
	if !strings.Contains(out, "disabled") {
		t.Fatalf("want 'disabled' after disable, got: %s", out)
	}
}

// TC-CTEL-04: status --json returns expected keys.
func TestTelemetry_Status_JSON(t *testing.T) {
	dir := t.TempDir()
	out := exec(t, []string{"status", "--root", dir, "--json"})
	var p map[string]any
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	for _, k := range []string{"enabled", "install_id"} {
		if _, ok := p[k]; !ok {
			t.Errorf("missing key %q in status JSON", k)
		}
	}
}

// TC-CTEL-05: rotate-id changes install_id.
func TestTelemetry_RotateID(t *testing.T) {
	dir := t.TempDir()
	out1 := exec(t, []string{"status", "--root", dir, "--json"})
	var p1 map[string]any
	json.Unmarshal([]byte(out1), &p1) //nolint:errcheck
	id1 := p1["install_id"].(string)

	exec(t, []string{"rotate-id", "--root", dir})

	out2 := exec(t, []string{"status", "--root", dir, "--json"})
	var p2 map[string]any
	json.Unmarshal([]byte(out2), &p2) //nolint:errcheck
	id2 := p2["install_id"].(string)

	if id1 == id2 {
		t.Fatalf("rotate-id did not change install_id: %q", id1)
	}
}

// TC-CTEL-06: enable persists across status calls.
func TestTelemetry_Persistence(t *testing.T) {
	dir := t.TempDir()
	exec(t, []string{"enable", "--root", dir})
	cfg, err := telemetry.LoadConfig(filepath.Join(dir, telemetry.DefaultConfigPath))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.Enabled {
		t.Error("want Enabled=true persisted")
	}
}

// TC-CTEL-07: idempotency — two status calls return same output.
func TestTelemetry_Idempotency(t *testing.T) {
	dir := t.TempDir()
	exec(t, []string{"enable", "--root", dir})
	out1 := exec(t, []string{"status", "--root", dir, "--json"})
	out2 := exec(t, []string{"status", "--root", dir, "--json"})
	if out1 != out2 {
		t.Fatalf("idempotency: outputs differ\n1: %s\n2: %s", out1, out2)
	}
}

// TC-CTEL-08: false-positive guard — disable after fresh dir shows disabled, not error.
func TestTelemetry_FalsePositive_DisableWhenAlreadyDisabled(t *testing.T) {
	dir := t.TempDir()
	out := exec(t, []string{"disable", "--root", dir})
	if !strings.Contains(out, "disabled") {
		t.Fatalf("false-positive: want 'disabled' output, got: %s", out)
	}
}
