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
package cmdexplain

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/verbmeta"
)

func TestExplain_ListsAllVerbs(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "explain") {
		t.Fatalf("listing missing self: %s", got)
	}
}

func TestExplain_OneVerbJSON(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"explain", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var m verbmeta.Manifest
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("not JSON: %v: %s", err, out.String())
	}
	if m.Verb != "explain" {
		t.Fatalf("unexpected manifest: %+v", m)
	}
}

func TestExplain_UnknownVerb(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"definitely-not-a-verb"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error, got: %s", out.String())
	}
	if !strings.Contains(err.Error(), "FORGE-1400") {
		t.Fatalf("want FORGE-1400, got: %v", err)
	}
}
