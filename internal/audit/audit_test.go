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
package audit

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func tmpLedger(t *testing.T) (*Ledger, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".forge", "audit.log")
	l, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return l, path
}

// TC-AUDIT-01 (happy + data-accuracy): single entry round-trips.
func TestAudit_AppendAndRead(t *testing.T) {
	t.Parallel()
	l, _ := tmpLedger(t)
	e, err := l.Append(Entry{Verb: "ship", Action: "checkpoint-pass", Actor: "alice"})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if e.Hash == "" {
		t.Fatal("entry has no hash")
	}
	if e.PrevHash != "" {
		t.Errorf("genesis prev_hash should be empty, got %q", e.PrevHash)
	}
	all, err := l.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("All len: got %d", len(all))
	}
	if all[0].Hash != e.Hash {
		t.Errorf("hash mismatch on read: %s vs %s", all[0].Hash, e.Hash)
	}
}

// TC-AUDIT-02 (happy): chain links correctly across multiple entries.
func TestAudit_HashChainLinks(t *testing.T) {
	t.Parallel()
	l, _ := tmpLedger(t)
	e1, _ := l.Append(Entry{Verb: "new", Action: "scaffold"})
	e2, _ := l.Append(Entry{Verb: "scan", Action: "secrets-clean"})
	e3, _ := l.Append(Entry{Verb: "ship", Action: "deploy"})

	if e2.PrevHash != e1.Hash {
		t.Errorf("e2.prev=%s, want %s", e2.PrevHash, e1.Hash)
	}
	if e3.PrevHash != e2.Hash {
		t.Errorf("e3.prev=%s, want %s", e3.PrevHash, e2.Hash)
	}
}

// TC-AUDIT-03 (data-accuracy): Verify returns -1 on intact chain.
func TestAudit_VerifyIntact(t *testing.T) {
	t.Parallel()
	l, _ := tmpLedger(t)
	for i := 0; i < 5; i++ {
		if _, err := l.Append(Entry{Verb: "scan", Action: "ok"}); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	idx, err := l.Verify()
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if idx != -1 {
		t.Fatalf("intact chain should yield -1, got %d", idx)
	}
}

// TC-AUDIT-04 (negative + regression): tampered entry is detected.
func TestAudit_TamperDetected(t *testing.T) {
	t.Parallel()
	l, path := tmpLedger(t)
	_, _ = l.Append(Entry{Verb: "a", Action: "1"})
	_, _ = l.Append(Entry{Verb: "b", Action: "2"})

	// Corrupt the file.
	body, _ := os.ReadFile(path)
	corrupt := []byte("{\"verb\":\"evil\",\"action\":\"injected\",\"prev_hash\":\"\",\"hash\":\"deadbeef\"}\n")
	if err := os.WriteFile(path, append(corrupt, body...), 0o600); err != nil {
		t.Fatal(err)
	}
	l2, _ := Open(path)
	idx, err := l2.Verify()
	if err == nil || idx != 0 {
		t.Fatalf("tamper not detected: idx=%d err=%v", idx, err)
	}
}

// TC-AUDIT-05 (boundary): Open of missing file is OK; first Append starts genesis.
func TestAudit_OpenMissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "audit.log")
	l, err := Open(path)
	if err != nil {
		t.Fatalf("Open missing: %v", err)
	}
	all, _ := l.All()
	if len(all) != 0 {
		t.Fatalf("missing file should give empty, got %d", len(all))
	}
	_, err = l.Append(Entry{Verb: "x", Action: "y"})
	if err != nil {
		t.Fatalf("Append after missing-Open: %v", err)
	}
}

// TC-AUDIT-06 (concurrency): parallel Appends are serialized + chain stays valid.
func TestAudit_ConcurrentAppendChainValid(t *testing.T) {
	t.Parallel()
	l, _ := tmpLedger(t)
	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()
			_, _ = l.Append(Entry{Verb: "x", Action: "concurrent"})
		}(i)
	}
	wg.Wait()
	idx, err := l.Verify()
	if err != nil {
		t.Fatalf("Verify after concurrent appends: %v idx=%d", err, idx)
	}
}
