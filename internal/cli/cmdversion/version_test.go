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
package cmdversion

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestVersion_Text(t *testing.T) {
	t.Parallel()
	cmd := New("9.9.9-test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "forge 9.9.9-test ") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestVersion_JSON(t *testing.T) {
	t.Parallel()
	cmd := New("9.9.9-test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v: %q", err, out.String())
	}
	if got["version"] != "9.9.9-test" {
		t.Fatalf("version = %q", got["version"])
	}
	for _, k := range []string{"go_version", "os", "arch"} {
		if got[k] == "" {
			t.Errorf("missing %s", k)
		}
	}
}
