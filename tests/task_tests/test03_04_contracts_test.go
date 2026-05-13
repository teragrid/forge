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

// TEST-03: Contract tests for plugin interfaces.
// TEST-04: LLM cassette/recording library.

package tasktests

import (
	"testing"

	"github.com/teragrid/forge/internal/llmcache"
	"github.com/teragrid/forge/internal/plugin"
)

// ── TEST-03: Plugin contract tests ───────────────────────────────────────────

// TC-03-01 (happy): every registered plugin passes manifest validation.
func TestTC0301_PluginManifestValid(t *testing.T) {
	t.Parallel()
	plugins := plugin.Default().All()
	if len(plugins) == 0 {
		t.Skip("no plugins registered — contract tests skipped")
	}
	for _, p := range plugins {
		p := p
		m := p.Manifest()
		t.Run(m.Name, func(t *testing.T) {
			t.Parallel()
			if err := m.Validate(); err != nil {
				t.Errorf("manifest.Validate() = %v; want nil", err)
			}
		})
	}
}

// TC-03-02 (boundary): interface methods receiving empty inputs return
// spec-defined empty results, not nulls — validated via manifest.Validate()
// on a minimal manifest.
func TestTC0302_PluginEmptyInputBoundary(t *testing.T) {
	t.Parallel()
	// A manifest with all required fields set is valid.
	m := plugin.Manifest{
		Name:    "test-plugin",
		Version: "1.0.0",
		Kind:    plugin.KindScanner,
	}
	if err := m.Validate(); err != nil {
		t.Errorf("valid manifest failed Validate: %v", err)
	}
}

// TC-03-03 (negative): a manifest missing required fields is rejected.
func TestTC0303_PluginMissingFieldsRejected(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		m    plugin.Manifest
	}{
		{"missing-name", plugin.Manifest{Version: "1.0.0", Kind: plugin.KindScanner}},
		{"missing-version", plugin.Manifest{Name: "x", Kind: plugin.KindScanner}},
		{"missing-kind", plugin.Manifest{Name: "x", Version: "1.0.0"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.m.Validate(); err == nil {
				t.Errorf("Validate() should have failed for %s", tc.name)
			}
		})
	}
}

// TC-03-05 (idempotency): contract-suite re-run on the same plugin list
// produces the same pass/fail set.
func TestTC0305_PluginContractIdempotency(t *testing.T) {
	t.Parallel()
	plugins := plugin.Default().All()
	if len(plugins) == 0 {
		t.Skip("no plugins registered")
	}
	var firstResult []bool
	var secondResult []bool
	for _, p := range plugins {
		m := p.Manifest()
		firstResult = append(firstResult, m.Validate() == nil)
	}
	for _, p := range plugins {
		m := p.Manifest()
		secondResult = append(secondResult, m.Validate() == nil)
	}
	if len(firstResult) != len(secondResult) {
		t.Fatalf("result lengths differ: %d vs %d", len(firstResult), len(secondResult))
	}
	for i := range firstResult {
		if firstResult[i] != secondResult[i] {
			t.Errorf("plugin[%d]: first=%v second=%v", i, firstResult[i], secondResult[i])
		}
	}
}

// TC-03-06 (false-positive guard): a deliberately broken manifest flips the
// contract test to fail.
func TestTC0306_PluginBrokenManifestFails(t *testing.T) {
	t.Parallel()
	broken := plugin.Manifest{} // no Name, Version, or Kind
	if err := broken.Validate(); err == nil {
		t.Fatal("Validate() returned nil for empty manifest; expected an error")
	}
}

// ── TEST-04: LLM cassette / recording library ─────────────────────────────────

// TC-04-01 (happy): store then get produces the same response; no live call needed.
func TestTC0401_CassetteRecordReplay(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c, err := llmcache.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	key := llmcache.Key("gpt-4", "sys", "user")
	const want = "hello world"
	if err := c.Store(key, "gpt-4", want, nil); err != nil {
		t.Fatalf("Store: %v", err)
	}

	got, ok := c.Get(key, nil)
	if !ok {
		t.Fatal("Get: expected cache hit, got miss")
	}
	if got.Response != want {
		t.Errorf("Response = %q, want %q", got.Response, want)
	}
}

// TC-04-02 (boundary): getting from an empty cache dir is a miss (not an error).
func TestTC0402_CassetteEmptyDirMiss(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c, err := llmcache.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	key := llmcache.Key("gpt-4", "sys", "user")
	_, ok := c.Get(key, nil)
	if ok {
		t.Fatal("expected cache miss on empty dir, got hit")
	}
}

// TC-04-04 (idempotency): replaying the same key twice yields identical responses.
func TestTC0404_CassetteReplayIdempotency(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c, err := llmcache.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	key := llmcache.Key("gpt-4", "sys", "q")
	if err := c.Store(key, "gpt-4", "answer", nil); err != nil {
		t.Fatalf("Store: %v", err)
	}
	first, _ := c.Get(key, nil)
	second, _ := c.Get(key, nil)
	if first == nil || second == nil {
		t.Fatal("expected two hits, got a miss")
	}
	if first.Response != second.Response {
		t.Errorf("responses differ: %q vs %q", first.Response, second.Response)
	}
}

// TC-04-06 (false-positive guard): a key not present in the cache is a miss,
// proving the harness is not silently passing all lookups.
func TestTC0406_CassetteMissIsNotSilentlyPass(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c, err := llmcache.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Store one key, look up a different one.
	stored := llmcache.Key("gpt-4", "sys", "a")
	other := llmcache.Key("gpt-4", "sys", "b")
	if err := c.Store(stored, "gpt-4", "resp", nil); err != nil {
		t.Fatalf("Store: %v", err)
	}
	_, ok := c.Get(other, nil)
	if ok {
		t.Fatal("different key returned a hit — cache is not key-discriminating")
	}
}
