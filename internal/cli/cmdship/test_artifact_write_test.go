package cmdship

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTestArtifact_KeepsFilesForgeDidNotWrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "src", "test", "feat.test.ts")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	// First write: file does not exist yet -> written and recorded.
	if !writeTestArtifact(root, "feat", path, "stub v1\n") {
		t.Fatal("first write must succeed")
	}

	// Replay with the same or new stub while the file is untouched -> overwritten.
	if !writeTestArtifact(root, "feat", path, "stub v2\n") {
		t.Fatal("an untouched forge-written file must be overwritable")
	}

	// The agent replaces the stub with real tests; a replay must not clobber them.
	if err := os.WriteFile(path, []byte("real tests\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if writeTestArtifact(root, "feat", path, "stub v3\n") {
		t.Fatal("a file edited after forge wrote it must be kept")
	}
	if got, _ := os.ReadFile(path); string(got) != "real tests\n" {
		t.Fatalf("edited file was overwritten: %q", got)
	}
}

func TestWriteTestArtifact_KeepsUnrecordedExistingFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "feat.rls.test.ts")
	if err := os.WriteFile(path, []byte("hand written\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if writeTestArtifact(root, "feat", path, "stub\n") {
		t.Fatal("an existing file with no forge record must be kept")
	}
}

func TestWriteTestArtifact_RecreatesDeletedFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "feat.integration.test.ts")
	writeTestArtifact(root, "feat", path, "stub\n")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if !writeTestArtifact(root, "feat", path, "stub\n") {
		t.Fatal("a deleted artifact must be written again")
	}
}

func TestStripOuterCodeFence(t *testing.T) {
	in := "```typescript\nimport { it } from 'jest';\nit('x', () => {});\n```\n"
	got := stripOuterCodeFence(in)
	if strings.Contains(got, "```") {
		t.Fatalf("fence not stripped: %q", got)
	}
	if !strings.HasPrefix(got, "import { it }") {
		t.Fatalf("body mangled: %q", got)
	}
	plain := "import x from 'y';\n"
	if stripOuterCodeFence(plain) != plain {
		t.Fatal("unfenced content must be returned unchanged")
	}
	inner := "const s = `a`;\n```js\nnot a wrapper\n"
	if stripOuterCodeFence(inner) != inner {
		t.Fatal("content that does not start with a fence must be returned unchanged")
	}
}

func TestADRAlternativeCount_CountsListUnderAlternativesHeading(t *testing.T) {
	adr := strings.ToLower(`# ADR
## Alternatives considered
1. Keep the 3-day grace — rejected.
2. Downgrade to a free tier — rejected.
3. Count from the webhook — rejected.
## Consequences
Fewer blocks.`)
	if n := adrAlternativeCount(adr); n < 2 {
		t.Fatalf("expected >= 2 alternatives, got %d", n)
	}

	single := strings.ToLower("# ADR\n## Decision\nWe do X.\n## Consequences\nY.")
	if n := adrAlternativeCount(single); n >= 2 {
		t.Fatalf("an ADR with no alternatives must stay below 2, got %d", n)
	}
}
