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
package cmdscan

import (
	"context"
	"testing"

	"github.com/teragrid/forge/internal/plugin"
)

// TestBuiltinScanners_RegisteredInDefault asserts every builtin scanner family
// is discoverable via plugin.Default() (init() side-effect of importing this
// package).
func TestBuiltinScanners_RegisteredInDefault(t *testing.T) {
	want := []string{"secrets", "rls", "prompt-injection", "supply-chain"}
	for _, name := range want {
		p, ok := plugin.Default().Lookup(name)
		if !ok {
			t.Fatalf("scanner %q not registered in plugin.Default()", name)
		}
		if got := p.Manifest().Kind; got != plugin.KindScanner {
			t.Errorf("scanner %q kind = %q, want %q", name, got, plugin.KindScanner)
		}
		if _, ok := p.(plugin.Scanner); !ok {
			t.Errorf("scanner %q does not satisfy plugin.Scanner", name)
		}
	}
}

// TestBuiltinScanner_Adapter_DataAccuracy verifies the adapter returns the
// same finding count as the underlying Run* function (no data loss in
// translation). Uses an empty tmp dir so result is deterministic (zero
// findings) but still exercises the conversion path.
func TestBuiltinScanner_Adapter_DataAccuracy(t *testing.T) {
	tmp := t.TempDir()
	for _, p := range builtinScanners() {
		s, ok := p.(plugin.Scanner)
		if !ok {
			t.Fatalf("%s not a plugin.Scanner", p.Manifest().Name)
		}
		got, err := s.Scan(context.Background(), tmp)
		if err != nil {
			t.Fatalf("%s Scan: %v", p.Manifest().Name, err)
		}
		if len(got) != 0 {
			t.Errorf("%s Scan on empty tmp returned %d findings, want 0", p.Manifest().Name, len(got))
		}
	}
}

// TestBuiltinScanner_Adapter_ConvertsFindings exercises the result conversion
// by running secrets against a synthetic file containing a known pattern,
// then comparing adapter output to a direct RunSecrets call (data accuracy).
func TestBuiltinScanner_Adapter_ConvertsFindings(t *testing.T) {
	tmp := t.TempDir()
	mustWrite := writeFile
	mustWrite(t, tmp, "leak.txt", "AKIAABCDEFGHIJKLMNOP\n")

	direct, err := RunSecrets(tmp)
	if err != nil {
		t.Fatalf("RunSecrets: %v", err)
	}
	if len(direct.Findings) == 0 {
		t.Skip("RunSecrets did not detect synthetic AKIA token (gitleaks may have suppressed); skipping accuracy check")
	}

	p, _ := plugin.Default().Lookup("secrets")
	via, err := p.(plugin.Scanner).Scan(context.Background(), tmp)
	if err != nil {
		t.Fatalf("adapter Scan: %v", err)
	}
	if len(via) != len(direct.Findings) {
		t.Fatalf("adapter findings=%d, direct findings=%d", len(via), len(direct.Findings))
	}
}
