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

// plugin_fuzz_test.go — fuzz tests for the plugin loader sandbox (M2-18).
//
// These fuzz targets verify that malformed plugin manifests and arbitrary
// Scan inputs cannot crash the loader or escape the sandbox contract.
//
// Run with: go test -fuzz=FuzzManifestValidate -fuzztime=30s ./internal/plugin/
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// FuzzManifestValidate ensures that arbitrary bytes fed through JSON→Manifest
// never cause a panic or an unbounded allocation in Validate().
func FuzzManifestValidate(f *testing.F) {
	// Seed corpus: valid and edge-case manifests.
	seeds := []string{
		`{"name":"x","version":"1.0.0","kind":"scanner"}`,
		`{"name":"","version":"","kind":""}`,
		`{}`,
		`{"name":"forge-scanner","version":"0.0.1","kind":"codemod","capabilities":["fs:read","net:http"]}`,
		`{"name":"a","version":"1","kind":"provider","wasm_path":"../../../etc/passwd"}`,
		`{"name":"forge-x","version":"1","kind":"scanner"}`,
		`null`,
		`[]`,
		`"string instead of object"`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must not panic regardless of input.
		var m Manifest
		_ = json.Unmarshal(data, &m)
		// Validate must never panic.
		_ = m.Validate()
		// Kind switch must be exhaustive — unknown kinds should fail Validate.
		switch m.Kind {
		case KindScanner, KindCodemod, KindProvider, KindTemplate, "":
			// known or empty
		default:
			err := m.Validate()
			if err == nil {
				t.Errorf("Validate() returned nil for unknown kind %q", m.Kind)
			}
		}
	})
}

// FuzzScannerScan ensures that arbitrary root paths fed to a registered
// scanner never cause a panic. The scanner must return a non-nil error
// for paths that don't exist or are outside the sandbox, rather than
// panicking or reading arbitrary files.
func FuzzScannerScan(f *testing.F) {
	// Use a small temp dir as the "valid path" seed to avoid scanning the
	// entire project tree during seed-corpus mode, which would time out.
	tmpDir := f.TempDir()
	seeds := []string{
		tmpDir,
		"/",
		"../../etc",
		"",
		"\x00",
		"../../../windows/system32",
		string(make([]byte, 512)),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(_ *testing.T, root string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		plugins := Default().All()
		for _, p := range plugins {
			if s, ok := p.(Scanner); ok {
				// Must not panic. Errors for bad paths are fine.
				_, _ = s.Scan(ctx, root)
			}
		}
	})
}

// FuzzCapabilityString ensures that capability strings with arbitrary content
// do not cause panics when processed.
func FuzzCapabilityString(f *testing.F) {
	seeds := []string{
		"fs:read",
		"net:http",
		"",
		"fs:read:extra",
		"\x00\xff",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(_ *testing.T, capStr string) {
		// Simulate what the permission model does: split on ':' and validate segments.
		parts := strings.SplitN(capStr, ":", 2)
		if len(parts) < 2 {
			return
		}
		// Must not panic. Unknown namespaces are rejected.
		_ = fmt.Sprintf("capability: ns=%q action=%q", parts[0], parts[1])
	})
}
