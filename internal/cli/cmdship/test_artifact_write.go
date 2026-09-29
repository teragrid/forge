package cmdship

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Test-artifact writes must never clobber work.
//
// Every `forge ship --agent-mode` re-run replays the recorded test turns and
// used to rewrite src/test/<slug>.* unconditionally — silently replacing the
// real tests an agent had written in place of the red stubs with the stubs
// again (seen twice in one ai-marketing-platfrom session, 2026-09-29: the
// only way to notice was a "file changed on disk" diff). forge now records the
// hash of every test artifact it writes in .forge/specs/<slug>/test-artifacts.json
// and only overwrites a file that is still byte-for-byte what forge wrote.
// A file that exists without a record (hand-written, or written by an older
// forge) is also kept.

const testArtifactManifestName = "test-artifacts.json"

func testArtifactManifestPath(root, slug string) string {
	return filepath.Join(root, ".forge", "specs", slug, testArtifactManifestName)
}

func loadTestArtifactManifest(root, slug string) map[string]string {
	m := map[string]string{}
	data, err := os.ReadFile(testArtifactManifestPath(root, slug))
	if err == nil {
		_ = json.Unmarshal(data, &m)
	}
	return m
}

func saveTestArtifactManifest(root, slug string, m map[string]string) {
	p := testArtifactManifestPath(root, slug)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	if data, err := json.MarshalIndent(m, "", "  "); err == nil {
		_ = os.WriteFile(p, append(data, '\n'), 0o600)
	}
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// stripOuterCodeFence removes a single markdown fence wrapping the whole
// answer (```typescript … ```). Agent-mode answers are recorded verbatim, and
// the fenced form is what the turn prompt asks for — written as-is it made
// the first line of <slug>.test.ts "```typescript", which does not compile.
func stripOuterCodeFence(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") {
		return s
	}
	nl := strings.IndexByte(t, '\n')
	if nl < 0 {
		return s
	}
	body := strings.TrimRight(t[nl+1:], " \t\r\n")
	if !strings.HasSuffix(body, "```") {
		return s
	}
	body = strings.TrimSuffix(body, "```")
	return strings.TrimRight(body, " \t\r\n") + "\n"
}

// writeTestArtifact writes content to path unless the file already holds
// something forge did not write. Returns true when the file was written.
func writeTestArtifact(root, slug, path, content string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)

	manifest := loadTestArtifactManifest(root, slug)
	if existing, err := os.ReadFile(path); err == nil {
		recorded, known := manifest[rel]
		if !known || recorded != sha256Hex(existing) {
			fmt.Fprintf(os.Stderr, "forge: kept %s (modified since forge generated it)\n", rel)
			return false
		}
	}

	data := []byte(content)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return false
	}
	manifest[rel] = sha256Hex(data)
	saveTestArtifactManifest(root, slug, manifest)
	return true
}
