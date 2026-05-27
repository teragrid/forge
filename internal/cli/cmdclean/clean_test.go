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
package cmdclean

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/manifest"
)

func setupTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite := func(rel, body string) {
		full := filepath.Join(root, rel)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Scratch candidates (default patterns).
	mustWrite(".forge/scratch/note.txt", "x")
	mustWrite("_scratch_temp", "x")
	mustWrite("draft.tmp.md", "x")
	// Legitimate (managed) files.
	mustWrite("README.md", "readme")
	mustWrite("LICENSE", "lic")
	mustWrite("src/main.go", "package main")
	return root
}

func TestRun_Check_FindsScratch(t *testing.T) {
	t.Parallel()
	root := setupTree(t)
	res, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Mode != "check" {
		t.Fatalf("mode = %q", res.Mode)
	}
	expectIncludes(t, res.Candidates, ".forge/scratch", "_scratch_temp", "draft.tmp.md")
	expectExcludes(t, res.Candidates, "README.md", "LICENSE", "src/main.go")
}

func TestRun_Apply_DeletesAndIdempotent(t *testing.T) {
	t.Parallel()
	root := setupTree(t)
	res, err := Run(root, true)
	if err != nil {
		t.Fatalf("Run apply: %v", err)
	}
	if len(res.Deleted) == 0 {
		t.Fatal("expected deletions")
	}
	for _, d := range res.Deleted {
		if _, err := os.Stat(filepath.Join(root, d)); err == nil {
			t.Errorf("%s still exists after apply", d)
		}
	}
	// Idempotency: second run should find nothing.
	res2, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run again: %v", err)
	}
	if len(res2.Candidates) != 0 {
		t.Fatalf("expected 0 candidates after apply, got %v", res2.Candidates)
	}
	// README must survive.
	if _, err := os.Stat(filepath.Join(root, "README.md")); err != nil {
		t.Fatalf("README.md must survive clean: %v", err)
	}
}

func TestRun_EmptyTree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	res, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Candidates) != 0 {
		t.Fatalf("empty tree should have no candidates: %v", res.Candidates)
	}
}

func TestRun_SkipsDotGit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".git", "objects"), 0o755)
	_ = os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref"), 0o600)
	_ = os.WriteFile(filepath.Join(root, "_scratch_x"), []byte("x"), 0o600)
	res, err := Run(root, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Candidates {
		if strings.HasPrefix(c, ".git") || strings.HasPrefix(c, ".git/") {
			t.Fatalf(".git must not appear in candidates: %v", c)
		}
	}
}

// ── DEV-M0-36 secret guard tests ───────────────────────────────────────────

// TC-36-01: checkTrackedSecrets in a non-git tree → graceful skip (no panic, empty result).
func TestCheckTrackedSecrets_NonGitTree(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// No .git directory; git ls-files should fail → err returned, caller skips.
	secrets, err := checkTrackedSecrets(dir, manifest.File{})
	if err == nil && len(secrets) > 0 {
		t.Errorf("expected no secrets in non-git tree, got %v", secrets)
	}
	// err != nil is expected and fine — caller suppresses it.
}

// TC-36-02: secretPatterns coverage — patterns that SHOULD match.
func TestSecretPatterns_Match(t *testing.T) {
	t.Parallel()
	shouldMatch := []string{
		".env", ".env.local", ".env.production.local",
		"server.pem", "private.key", "id_rsa", "id_ed25519", "id_ecdsa", "id_dsa",
		"secrets.json", "credentials.json", ".netrc",
	}
	for _, name := range shouldMatch {
		matched := false
		for _, pat := range secretPatterns {
			if ok, _ := filepath.Match(pat, name); ok {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("expected %q to match a secret pattern but it did not", name)
		}
	}
}

// TC-36-03 false-positive guard: names that MUST NOT match.
func TestSecretPatterns_NoFalsePositive(t *testing.T) {
	t.Parallel()
	shouldNotMatch := []string{
		".env.local.example",
		".env.example",
		"environment.go",
		"key_handler.go",
		"secrets_test.go",
		"README.md",
		"main.go",
		".envrc",
	}
	for _, name := range shouldNotMatch {
		for _, pat := range secretPatterns {
			if ok, _ := filepath.Match(pat, name); ok {
				t.Errorf("false positive: %q matched secret pattern %q — should NOT match", name, pat)
			}
		}
	}
}

// TC-36-04: Run() with git unavailable does NOT return an error (graceful degradation).
func TestRun_SecretGuard_GracefulSkipWhenNoGit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Write .env inside the tree — no git repo.
	_ = os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=x"), 0o600)
	// Run should succeed (no git = graceful skip).
	res, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run should not error when git is absent: %v", err)
	}
	// TrackedSecrets should be nil or empty since git is unavailable.
	if len(res.TrackedSecrets) > 0 {
		t.Errorf("expected no TrackedSecrets without git repo, got %v", res.TrackedSecrets)
	}
}

// TC-36-05 (integration): Run() with a real git repo that has a tracked .env file
// detects it in TrackedSecrets. Skipped if git binary is unavailable.
func TestRun_SecretGuard_GitTrackedEnvFile(t *testing.T) {
	t.Parallel()

	// Require git to be available.
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git binary not found — skipping integration test")
	}

	root := t.TempDir()

	// Initialise a bare git repo.
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(git, args...)
		cmd.Dir = root
		// Set minimal git config so commit works in clean CI environments.
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@forge.test",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@forge.test",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@forge.test")
	run("config", "user.name", "Forge Test")

	// Write and commit a .env file — this is the secret file we want detected.
	envPath := filepath.Join(root, ".env")
	if err := os.WriteFile(envPath, []byte("API_KEY=hunter2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".env")
	run("commit", "-m", "chore: seed test repo with .env")

	// Run forge clean (check mode — no deletions).
	res, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	found := false
	for _, s := range res.TrackedSecrets {
		if s == ".env" || strings.HasSuffix(filepath.ToSlash(s), "/.env") || s == ".env" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected .env in TrackedSecrets, got %v", res.TrackedSecrets)
	}
}

func expectIncludes(t *testing.T, got []string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		found := false
		for _, g := range got {
			if g == w || strings.HasPrefix(g, w+"/") || strings.HasPrefix(g, w) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected candidate %q in %v", w, got)
		}
	}
}

func expectExcludes(t *testing.T, got []string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		for _, g := range got {
			if g == w {
				t.Errorf("did NOT expect candidate %q (in %v)", w, got)
			}
		}
	}
}

// ── G-061: dry-run and trash modes ────────────────────────────────────────────

// TestRunDryRun_ShowsButNoDelete verifies that RunDryRun reports candidates
// without deleting them from disk. G-061.
func TestRunDryRun_ShowsButNoDelete(t *testing.T) {
	t.Parallel()
	root := setupTree(t)
	res, err := RunDryRun(root)
	if err != nil {
		t.Fatalf("RunDryRun: %v", err)
	}
	if res.Mode != "dry-run" {
		t.Errorf("mode = %q, want dry-run", res.Mode)
	}
	if len(res.Candidates) == 0 {
		t.Fatal("expected candidates in dry-run mode")
	}
	if len(res.Deleted) != 0 {
		t.Errorf("dry-run must not delete files, got Deleted=%v", res.Deleted)
	}
	// Files must still exist on disk.
	for _, c := range res.Candidates {
		if _, err := os.Stat(filepath.Join(root, c)); err != nil {
			t.Errorf("dry-run deleted %s — must not delete", c)
		}
	}
}

// TestRunWithTrash_MovesFiles verifies that RunWithTrash moves candidates to
// .forge/trash/<run-id>/ and they no longer exist at their original paths. G-061.
func TestRunWithTrash_MovesFiles(t *testing.T) {
	t.Parallel()
	root := setupTree(t)
	res, err := RunWithTrash(root)
	if err != nil {
		t.Fatalf("RunWithTrash: %v", err)
	}
	if res.Mode != "apply" {
		t.Errorf("mode = %q, want apply", res.Mode)
	}
	if len(res.Deleted) == 0 {
		t.Fatal("expected deleted candidates")
	}
	if res.TrashDir == "" {
		t.Error("TrashDir must be set when files are moved to trash")
	}
	// Originals must be gone.
	for _, d := range res.Deleted {
		if _, err := os.Stat(filepath.Join(root, d)); err == nil {
			t.Errorf("%s should not exist at original location after RunWithTrash", d)
		}
	}
	// Managed files must survive.
	if _, err := os.Stat(filepath.Join(root, "README.md")); err != nil {
		t.Errorf("README.md must survive RunWithTrash: %v", err)
	}
}

// TestRunWithTrash_Recoverable verifies that the files moved to trash are
// actually readable at their new location (recoverable). G-061.
func TestRunWithTrash_Recoverable(t *testing.T) {
	t.Parallel()
	root := setupTree(t)
	res, err := RunWithTrash(root)
	if err != nil {
		t.Fatalf("RunWithTrash: %v", err)
	}
	if res.TrashDir == "" {
		t.Skip("no trash dir — nothing to verify")
	}
	trashBase := filepath.Join(root, filepath.FromSlash(res.TrashDir))
	// At least one deleted file must exist inside the trash directory.
	foundInTrash := false
	for _, d := range res.Deleted {
		trashPath := filepath.Join(trashBase, filepath.FromSlash(d))
		if _, err := os.Stat(trashPath); err == nil {
			foundInTrash = true
			break
		}
	}
	if !foundInTrash {
		t.Errorf("no deleted file found in trash dir %s — files are not recoverable", res.TrashDir)
	}
}

// TestRunWithTrash_EmptyTree returns empty result without error. G-061.
func TestRunWithTrash_EmptyTree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	res, err := RunWithTrash(root)
	if err != nil {
		t.Fatalf("RunWithTrash on empty tree: %v", err)
	}
	if len(res.Candidates) != 0 {
		t.Errorf("empty tree: expected 0 candidates, got %v", res.Candidates)
	}
	if res.TrashDir != "" {
		t.Error("TrashDir should be empty when no candidates found")
	}
}

// ── Cobra command integration ─────────────────────────────────────────────────

// TestNew_MutuallyExclusiveFlags verifies that combining --check and --apply
// returns an error (G-061: only one mode flag is allowed at a time).
func TestNew_MutuallyExclusiveFlags(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--root", root, "--check", "--apply"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when both --check and --apply are passed")
	}
}

// TestNew_JSONOutput verifies that --json produces valid JSON containing the
// expected Result fields (G-061).
func TestNew_JSONOutput(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--root", root, "--check", "--json"})
	// Execute may return a non-nil error if candidates are found; that is fine.
	_ = cmd.Execute()
	body := bytes.TrimSpace(out.Bytes())
	var result Result
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if result.Mode == "" {
		t.Error("expected non-empty Mode in JSON output")
	}
}

// ── Issue #15: loadMerged + autoSkipDirs ──────────────────────────────────────

// writeManifest writes a minimal .forge/manifest or hygiene.yml to dir.
func writeManifestFile(t *testing.T, path string, scratch, managed []string) {
	t.Helper()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	var b strings.Builder
	b.WriteString("[scratch]\n")
	for _, p := range scratch {
		b.WriteString(p + "\n")
	}
	b.WriteString("[managed]\n")
	for _, p := range managed {
		b.WriteString(p + "\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TC-15-01 (regression): hygiene.yml pattern **/fix_* matches fix_test.py
// at project root. Before the fix this was NOT detected; after it IS.
func TestLoadMerged_HygienePatternPickedUp(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Only hygiene.yml has "fix_*" pattern; the main manifest does not.
	writeManifestFile(t, filepath.Join(root, ".forge", "manifest"), []string{"_scratch_*"}, []string{"README.md"})
	writeManifestFile(t, filepath.Join(root, ".forge", "hygiene.yml"), []string{"fix_*"}, nil)
	// Create a file that matches only the hygiene.yml pattern.
	_ = os.WriteFile(filepath.Join(root, "fix_broken.py"), []byte("# fix"), 0o600)
	_ = os.WriteFile(filepath.Join(root, "README.md"), []byte("readme"), 0o600)

	res, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// fix_broken.py MUST appear (regression guard: pre-fix code would miss it).
	expectIncludes(t, res.Candidates, "fix_broken.py")
	// README must NOT appear.
	expectExcludes(t, res.Candidates, "README.md")
}

// TC-15-02 (happy path): patterns from both files are unioned; candidates
// from both sources are found in a single Run.
func TestLoadMerged_UnionsBothFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeManifestFile(t, filepath.Join(root, ".forge", "manifest"), []string{"_scratch_*"}, nil)
	writeManifestFile(t, filepath.Join(root, ".forge", "hygiene.yml"), []string{"patch_*"}, nil)
	_ = os.WriteFile(filepath.Join(root, "_scratch_note"), []byte("x"), 0o600)
	_ = os.WriteFile(filepath.Join(root, "patch_hack.go"), []byte("x"), 0o600)
	_ = os.WriteFile(filepath.Join(root, "main.go"), []byte("x"), 0o600)

	res, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	expectIncludes(t, res.Candidates, "_scratch_note", "patch_hack.go")
	expectExcludes(t, res.Candidates, "main.go")
}

// TC-15-03 (boundary): only hygiene.yml exists (no .forge/manifest) — hygiene
// patterns are used and manifest.Default() gracefully applies.
func TestLoadMerged_OnlyHygieneYML(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeManifestFile(t, filepath.Join(root, ".forge", "hygiene.yml"), []string{"tmp_*"}, nil)
	_ = os.WriteFile(filepath.Join(root, "tmp_debug.log"), []byte("x"), 0o600)

	res, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// tmp_debug.log should appear via hygiene.yml pattern.
	expectIncludes(t, res.Candidates, "tmp_debug.log")
}

// TC-15-04 (boundary): only .forge/manifest exists, no hygiene.yml — same
// behaviour as before the fix.
func TestLoadMerged_OnlyManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeManifestFile(t, filepath.Join(root, ".forge", "manifest"), []string{"_scratch_*"}, nil)
	_ = os.WriteFile(filepath.Join(root, "_scratch_old"), []byte("x"), 0o600)
	_ = os.WriteFile(filepath.Join(root, "keep.go"), []byte("x"), 0o600)

	res, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	expectIncludes(t, res.Candidates, "_scratch_old")
	expectExcludes(t, res.Candidates, "keep.go")
}

// TC-15-05 (false-positive guard): file inside node_modules/ must NOT be
// reported even when a broad pattern like _* matches its basename.
func TestAutoSkipDirs_NodeModules(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeManifestFile(t, filepath.Join(root, ".forge", "manifest"), []string{"_*"}, nil)
	_ = os.MkdirAll(filepath.Join(root, "node_modules", "@swc", "helpers", "cjs"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "node_modules", "@swc", "helpers", "cjs", "_async.cjs"), []byte("x"), 0o600)
	// A real scratch file at project root should still appear.
	_ = os.WriteFile(filepath.Join(root, "_scratch_note"), []byte("x"), 0o600)

	res, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, c := range res.Candidates {
		if strings.HasPrefix(c, "node_modules") {
			t.Errorf("node_modules file must not appear: %s", c)
		}
	}
	expectIncludes(t, res.Candidates, "_scratch_note")
}

// TC-15-06 (false-positive guard): file inside dist/ must NOT be reported.
func TestAutoSkipDirs_Dist(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeManifestFile(t, filepath.Join(root, ".forge", "manifest"), []string{"_*"}, nil)
	_ = os.MkdirAll(filepath.Join(root, "dist"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "dist", "_bundle.js"), []byte("x"), 0o600)

	res, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, c := range res.Candidates {
		if strings.HasPrefix(c, "dist") {
			t.Errorf("dist file must not appear: %s", c)
		}
	}
}

// TC-15-07 (idempotency): run loadMerged twice on same dir → same result.
func TestLoadMerged_Idempotent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeManifestFile(t, filepath.Join(root, ".forge", "manifest"), []string{"_scratch_*"}, nil)
	writeManifestFile(t, filepath.Join(root, ".forge", "hygiene.yml"), []string{"fix_*"}, nil)

	mf1, err1 := loadMerged(root)
	mf2, err2 := loadMerged(root)
	if err1 != nil || err2 != nil {
		t.Fatalf("loadMerged errors: %v / %v", err1, err2)
	}
	if len(mf1.Scratch) != len(mf2.Scratch) || len(mf1.Managed) != len(mf2.Managed) {
		t.Errorf("loadMerged not idempotent: first=%+v, second=%+v", mf1, mf2)
	}
}

// TC-15-08 (boundary): neither file exists → loadMerged returns manifest.Default()
// without error, and Run succeeds on an empty tree.
func TestLoadMerged_NeitherFileExists(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	res, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run on empty tree without manifests: %v", err)
	}
	if len(res.Candidates) != 0 {
		t.Errorf("expected 0 candidates on empty tree, got %v", res.Candidates)
	}
}
