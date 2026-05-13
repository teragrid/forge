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

package fssandbox_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/fssandbox"
)

func makeTree(t *testing.T) (root string, cleanup func()) {
	t.Helper()
	root = t.TempDir()
	// regular file
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	// sub-dir + file
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "world.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, func() {}
}

// ── Happy path ────────────────────────────────────────────────────────────────

func TestNew_Happy(t *testing.T) {
	t.Parallel()
	root, _ := makeTree(t)
	sb, err := fssandbox.New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if sb.Root() != filepath.Clean(root) {
		t.Fatalf("Root mismatch")
	}
}

func TestAbs_Happy(t *testing.T) {
	t.Parallel()
	root, _ := makeTree(t)
	sb, _ := fssandbox.New(root)

	abs, err := sb.Abs("hello.txt")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	if !strings.HasPrefix(abs, root) {
		t.Fatalf("Abs %q not under root %q", abs, root)
	}
}

func TestReadFile_Happy(t *testing.T) {
	t.Parallel()
	root, _ := makeTree(t)
	sb, _ := fssandbox.New(root)

	data, err := sb.ReadFile("hello.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("got %q, want hello", data)
	}
}

func TestGlob_Happy(t *testing.T) {
	t.Parallel()
	root, _ := makeTree(t)
	sb, _ := fssandbox.New(root)

	matches, err := sb.Glob("*.txt")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("want 1 match, got %d: %v", len(matches), matches)
	}
}

func TestWalk_Happy(t *testing.T) {
	t.Parallel()
	root, _ := makeTree(t)
	sb, _ := fssandbox.New(root)

	var files []string
	if err := sb.Walk(func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("want 2 files, got %d: %v", len(files), files)
	}
}

// ── Escape attempts ───────────────────────────────────────────────────────────

func TestAbs_EscapeRelative(t *testing.T) {
	t.Parallel()
	root, _ := makeTree(t)
	sb, _ := fssandbox.New(root)

	_, err := sb.Abs("../../etc/passwd")
	if err == nil {
		t.Fatal("expected ErrEscape, got nil")
	}
	if !strings.Contains(err.Error(), "FORGE-2500") {
		t.Fatalf("want FORGE-2500 in error, got: %v", err)
	}
}

func TestReadFile_EscapeRelative(t *testing.T) {
	t.Parallel()
	root, _ := makeTree(t)
	sb, _ := fssandbox.New(root)

	_, err := sb.ReadFile("../outside.txt")
	if err == nil {
		t.Fatal("expected ErrEscape for ../outside.txt")
	}
}

// ── Boundary cases ────────────────────────────────────────────────────────────

func TestAbs_RootItself(t *testing.T) {
	t.Parallel()
	root, _ := makeTree(t)
	sb, _ := fssandbox.New(root)

	abs, err := sb.Abs(".")
	if err != nil {
		t.Fatalf("Abs(.) should succeed: %v", err)
	}
	if abs != filepath.Clean(root) {
		t.Fatalf("Abs(.) = %q, want %q", abs, root)
	}
}

func TestNew_RootNotExist(t *testing.T) {
	t.Parallel()
	_, err := fssandbox.New("/does/not/exist/forge-sandbox-test-xyz")
	if err == nil {
		t.Fatal("expected error for non-existent root")
	}
}

func TestNew_RootIsFile(t *testing.T) {
	t.Parallel()
	root, _ := makeTree(t)
	_, err := fssandbox.New(filepath.Join(root, "hello.txt"))
	if err == nil {
		t.Fatal("expected error when root is a file")
	}
}

// ── False-positive guard ──────────────────────────────────────────────────────

// A path that shares the root's prefix but is outside must still be blocked.
// e.g. if root=/tmp/sandbox, path /tmp/sandbox2/x must be rejected.
func TestAbs_PrefixNotEnough(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	inside := filepath.Join(parent, "sandbox")
	sibling := filepath.Join(parent, "sandbox2")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}

	sb, err := fssandbox.New(inside)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt: ../sandbox2/file should escape
	_, err = sb.Abs("../sandbox2/evil.txt")
	if err == nil {
		t.Fatal("expected ErrEscape for sibling path, got nil")
	}
}
