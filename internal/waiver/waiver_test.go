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

package waiver

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeWaiver(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIsWaived_Hit(t *testing.T) {
	dir := t.TempDir()
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	writeWaiver(t, dir, "w001.yml", `
- id: W-001
  rule_id: SEC-001
  rationale: "accepted risk"
  approved_by: alice
  expires_at: "`+tomorrow+`"
`)
	r, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := r.IsWaived("SEC-001", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected waiver hit")
	}
}

func TestIsWaived_Miss(t *testing.T) {
	dir := t.TempDir()
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	writeWaiver(t, dir, "w001.yml", `
- id: W-001
  rule_id: SEC-001
  rationale: "accepted"
  approved_by: alice
  expires_at: "`+tomorrow+`"
`)
	r, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := r.IsWaived("SEC-999", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected waiver miss for different rule")
	}
}

func TestIsWaived_Expired(t *testing.T) {
	dir := t.TempDir()
	yesterday := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	writeWaiver(t, dir, "w_expired.yml", `
- id: W-EXP
  rule_id: PERF-001
  rationale: "temporary"
  approved_by: bob
  expires_at: "`+yesterday+`"
`)
	r, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := r.IsWaived("PERF-001", "")
	if err == nil {
		t.Fatal("expected ErrWaiverExpired")
	}
	if ok {
		t.Fatal("expired waiver must not count as waived")
	}
}

func TestIsWaived_FilePathScope(t *testing.T) {
	dir := t.TempDir()
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	writeWaiver(t, dir, "scoped.yml", `
- id: W-SCOPE
  rule_id: SEC-002
  file_path: "internal/foo/bar.go"
  rationale: "scoped"
  approved_by: alice
  expires_at: "`+tomorrow+`"
`)
	r, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Correct file → hit.
	ok, err := r.IsWaived("SEC-002", "internal/foo/bar.go")
	if err != nil || !ok {
		t.Fatalf("expected scoped hit, err=%v ok=%v", err, ok)
	}
	// Different file → miss.
	ok, err = r.IsWaived("SEC-002", "internal/other/baz.go")
	if err != nil || ok {
		t.Fatalf("expected scoped miss, err=%v ok=%v", err, ok)
	}
}

func TestEmptyDirIsOK(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ok, err := r.IsWaived("ANY-001", "")
	if err != nil || ok {
		t.Fatalf("empty registry should return (false, nil), got ok=%v err=%v", ok, err)
	}
}

func TestMissingDirIsOK(t *testing.T) {
	r, err := Load(filepath.Join(t.TempDir(), "nonexistent"))
	if err != nil {
		t.Fatal(err)
	}
	if r == nil {
		t.Fatal("expected non-nil registry for missing dir")
	}
}
