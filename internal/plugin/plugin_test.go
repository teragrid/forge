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
package plugin

import (
	"context"
	"testing"
)

type stubScanner struct {
	m Manifest
	f []Finding
}

func (s *stubScanner) Manifest() Manifest                                  { return s.m }
func (s *stubScanner) Scan(_ context.Context, _ string) ([]Finding, error) { return s.f, nil }

// Compile-time interface check (without err: simplified).
type stubCodemod struct{ m Manifest }

func (c *stubCodemod) Manifest() Manifest { return c.m }
func (c *stubCodemod) Apply(_ context.Context, _ string, dry bool) (Result, error) {
	return Result{Changed: 1, DryRun: dry}, nil
}

func newScanner(name string) *stubScanner {
	return &stubScanner{m: Manifest{Name: name, Version: "1.0.0", Kind: KindScanner}}
}
func newCodemod(name string) *stubCodemod {
	return &stubCodemod{m: Manifest{Name: name, Version: "1.0.0", Kind: KindCodemod}}
}

// TC-PLUGIN-01 (happy): register + lookup.
func TestRegistry_RegisterLookup(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register(newScanner("s1"))
	if _, ok := r.Lookup("s1"); !ok {
		t.Fatal("Lookup failed for s1")
	}
}

// TC-PLUGIN-02 (negative): duplicate panics.
func TestRegistry_DuplicatePanics(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register(newScanner("dup"))
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	r.Register(newScanner("dup"))
}

// TC-PLUGIN-03 (negative): invalid manifest panics.
func TestRegistry_InvalidManifestPanics(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on invalid manifest")
		}
	}()
	r.Register(&stubScanner{m: Manifest{Version: "1", Kind: KindScanner}}) // no name
}

// TC-PLUGIN-04 (data-accuracy): All() returns sorted.
func TestRegistry_AllSorted(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register(newScanner("zeta"))
	r.Register(newScanner("alpha"))
	r.Register(newCodemod("mike"))
	all := r.All()
	if len(all) != 3 {
		t.Fatalf("All len: got %d", len(all))
	}
	if all[0].Manifest().Name != "alpha" || all[2].Manifest().Name != "zeta" {
		t.Errorf("not sorted: %q %q %q", all[0].Manifest().Name, all[1].Manifest().Name, all[2].Manifest().Name)
	}
}

// TC-PLUGIN-05 (false-positive guard): ByKind filters correctly.
func TestRegistry_ByKindFilter(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register(newScanner("s1"))
	r.Register(newCodemod("c1"))
	scs := r.ByKind(KindScanner)
	if len(scs) != 1 || scs[0].Manifest().Name != "s1" {
		t.Errorf("scanner filter: %+v", scs)
	}
	cms := r.ByKind(KindCodemod)
	if len(cms) != 1 || cms[0].Manifest().Name != "c1" {
		t.Errorf("codemod filter: %+v", cms)
	}
}

// TC-PLUGIN-06 (boundary): Validate kind values.
func TestManifest_Validate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		m    Manifest
		ok   bool
	}{
		{"good", Manifest{Name: "x", Version: "1", Kind: KindScanner}, true},
		{"no name", Manifest{Version: "1", Kind: KindScanner}, false},
		{"no version", Manifest{Name: "x", Kind: KindScanner}, false},
		{"bad kind", Manifest{Name: "x", Version: "1", Kind: "bogus"}, false},
	}
	for _, c := range cases {
		err := c.m.Validate()
		if (err == nil) != c.ok {
			t.Errorf("%s: got err=%v want ok=%v", c.name, err, c.ok)
		}
	}
}

// TC-PLUGIN-07 (regression): default registry is shared.
func TestDefault_Shared(t *testing.T) {
	t.Parallel()
	if Default() == nil {
		t.Fatal("Default() nil")
	}
	d1 := Default()
	d2 := Default()
	if d1 != d2 {
		t.Fatal("Default() not stable")
	}
}

// ── G-100: WASM sandbox denies execution in stub build ────────────────────────

// TestPluginSandbox_WASM verifies that the WASM sandbox rejects plugin execution
// when the forge_wasm build tag is absent. In stub builds, any Call returns
// ErrNotLoaded — the tightest possible sandbox: no execution path exists.
func TestPluginSandbox_WASM(t *testing.T) {
	t.Parallel()
	m := Manifest{
		Name:     "untrusted-plugin",
		Version:  "1.0.0",
		Kind:     KindScanner,
		WASMPath: "/tmp/untrusted.wasm",
	}
	ep := NewExternalPlugin(m)
	_, err := ep.Call(context.Background(), []string{"--output=/etc/passwd"})
	if err == nil {
		t.Fatal("sandbox should deny execution; got nil error")
	}
	if err != ErrNotLoaded {
		t.Errorf("expected ErrNotLoaded from sandbox, got: %v", err)
	}
}

// ── G-130: WASM sandbox capability model ─────────────────────────────────────

// TestPlugin_WASMSandbox verifies that no capability beyond what the manifest
// declares can be exercised through the sandbox. In stub builds the constraint
// is absolute: every call returns ErrNotLoaded regardless of arguments,
// including attempts at filesystem escape or command injection.
func TestPlugin_WASMSandbox(t *testing.T) {
	t.Parallel()
	m := Manifest{
		Name:     "capability-test",
		Version:  "1.0.0",
		Kind:     KindCodemod,
		WASMPath: "plugins/capability-test.wasm",
	}
	ep := NewExternalPlugin(m)

	dangerousArgs := []struct {
		label string
		args  []string
	}{
		{"root-escape", []string{"--root=/"}},
		{"path-traversal", []string{"--output=../../etc/crontab"}},
		{"cmd-injection", []string{"/bin/sh", "-c", "rm -rf /"}},
		{"nil-args", nil},
	}

	for _, tc := range dangerousArgs {
		tc := tc
		t.Run(tc.label, func(t *testing.T) {
			t.Parallel()
			_, err := ep.Call(context.Background(), tc.args)
			if err == nil {
				t.Errorf("sandbox: expected error for args %v, got nil", tc.args)
			}
			if err != ErrNotLoaded {
				t.Errorf("sandbox: expected ErrNotLoaded for args %v, got: %v", tc.args, err)
			}
		})
	}
}
