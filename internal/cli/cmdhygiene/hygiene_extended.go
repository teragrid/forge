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

// Package cmdhygiene — G-060/G-061/G-064/G-065 hygiene extensions.
//
// G-060: HygieneManifest typed struct (loaded from .forge/hygiene.yml).
// G-061: forge clean --check / --dry-run / --apply mode support.
// G-064: RunHygieneCheck() called between Code and Ship checkpoints.
// G-065: EnsureGitignoreBaseline() writes minimum .gitignore entries.
package cmdhygiene

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ── G-060: HygieneManifest ────────────────────────────────────────────────────

// HygieneManifest is the typed representation of .forge/hygiene.yml.
type HygieneManifest struct {
	// Scratch lists glob patterns for files that may be safely deleted by
	// `forge clean --apply`.
	Scratch []string `yaml:"scratch"`
	// Managed lists glob patterns for files that are managed by Forge and
	// must not be deleted.
	Managed []string `yaml:"managed"`
	// RequiredFiles lists specific file paths that must exist; reported as
	// hygiene violations if absent.
	RequiredFiles []string `yaml:"required_files,omitempty"`
}

// LoadHygieneManifest reads and parses .forge/hygiene.yml from root.
// Returns an empty manifest (not an error) if the file does not exist.
func LoadHygieneManifest(root string) (*HygieneManifest, error) {
	path := filepath.Join(root, ".forge", "hygiene.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &HygieneManifest{}, nil
		}
		return nil, fmt.Errorf("load hygiene.yml: %w", err)
	}
	var m HygieneManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse hygiene.yml: %w", err)
	}
	return &m, nil
}

// SaveHygieneManifest writes the manifest to .forge/hygiene.yml.
func SaveHygieneManifest(root string, m *HygieneManifest) error {
	dir := filepath.Join(root, ".forge")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir .forge: %w", err)
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal hygiene.yml: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "hygiene.yml"), data, 0o600)
}

// ── G-061: CleanMode ─────────────────────────────────────────────────────────

// CleanMode controls the behaviour of RunClean.
type CleanMode string

const (
	// CleanCheck reports scratch files and exits non-zero if any found. CI-safe.
	CleanCheck CleanMode = "check"
	// CleanDryRun lists files that would be deleted but does not delete them.
	CleanDryRun CleanMode = "dry-run"
	// CleanApply deletes all files matching scratch patterns that are not in
	// the managed list and are not tracked by git.
	CleanApply CleanMode = "apply"
)

// CleanResult summarises the outcome of RunClean.
type CleanResult struct {
	Mode    CleanMode
	Found   []string // files matching scratch patterns
	Deleted []string // files actually deleted (CleanApply only)
	Errors  []string // per-file errors
}

// RunClean executes the clean operation in the given mode.
//
// It walks root for files matching any scratch pattern in the manifest,
// excludes any matching managed pattern, and then acts according to mode.
func RunClean(root string, m *HygieneManifest, mode CleanMode) (*CleanResult, error) {
	res := &CleanResult{Mode: mode}
	if len(m.Scratch) == 0 {
		return res, nil
	}

	// Collect candidate files.
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if matchesAny(rel, m.Scratch) && !matchesAny(rel, m.Managed) {
			res.Found = append(res.Found, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}

	switch mode {
	case CleanCheck:
		// Nothing to do — caller checks len(res.Found).
	case CleanDryRun:
		fmt.Printf("forge clean (dry-run): %d file(s) would be deleted:\n", len(res.Found))
		for _, f := range res.Found {
			fmt.Printf("  %s\n", f)
		}
	case CleanApply:
		for _, rel := range res.Found {
			abs := filepath.Join(root, rel)
			if rmErr := os.Remove(abs); rmErr != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", rel, rmErr))
			} else {
				res.Deleted = append(res.Deleted, rel)
			}
		}
		fmt.Printf("forge clean: deleted %d file(s)\n", len(res.Deleted))
	}
	return res, nil
}

// matchesAny returns true if path matches any of the given glob patterns.
func matchesAny(path string, patterns []string) bool {
	for _, p := range patterns {
		if ok, _ := filepath.Match(p, path); ok {
			return true
		}
		// Also match basename.
		base := filepath.Base(path)
		if ok, _ := filepath.Match(p, base); ok {
			return true
		}
	}
	return false
}

// ── G-064: hygiene check (Code→Ship checkpoint) ───────────────────────────────

// HygieneCheckResult is the result of RunHygieneCheck.
type HygieneCheckResult struct {
	ScratchFiles    []string
	MissingRequired []string
	Passed          bool
}

// RunHygieneCheck runs a non-destructive hygiene check. It:
//  1. Lists files matching scratch patterns.
//  2. Lists required files that are missing.
//
// Returns a summary that can be embedded in the ship pipeline checkpoint log.
func RunHygieneCheck(root string) (*HygieneCheckResult, error) {
	m, err := LoadHygieneManifest(root)
	if err != nil {
		return nil, err
	}
	res := &HygieneCheckResult{}

	// Check scratch files.
	cleanRes, err := RunClean(root, m, CleanCheck)
	if err == nil {
		res.ScratchFiles = cleanRes.Found
	}

	// Check required files.
	for _, req := range m.RequiredFiles {
		if _, statErr := os.Stat(filepath.Join(root, req)); os.IsNotExist(statErr) {
			res.MissingRequired = append(res.MissingRequired, req)
		}
	}

	res.Passed = len(res.ScratchFiles) == 0 && len(res.MissingRequired) == 0
	return res, nil
}

// ── G-065: .gitignore baseline ───────────────────────────────────────────────

// gitignoreBaseline is the minimum set of entries that forge adds to .gitignore.
// G-065: extended baseline.
var gitignoreBaseline = []string{
	// Forge internals.
	".forge/cache/",
	".forge/session/",
	".forge/learned/",
	".forge/scan-history/",
	".forge/llm-scratch/",
	".forge/trash/",
	".forge/eval-runs/",
	// Environment / secrets.
	".env",
	".env.local",
	".env.*.local",
	// OS and editor artefacts.
	".DS_Store",
	"Thumbs.db",
	"*.swp",
	// Dependency directories.
	"node_modules/",
	// Build / runtime logs.
	"*.log",
}

// EnsureGitignoreBaseline ensures that .gitignore at root contains the minimum
// forge-specific entries. Entries that are already present are not duplicated.
// The file is created if it does not exist.
func EnsureGitignoreBaseline(root string) error {
	path := filepath.Join(root, ".gitignore")

	// Read existing content.
	existing := map[string]bool{}
	if data, err := os.ReadFile(path); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			existing[strings.TrimSpace(scanner.Text())] = true
		}
	}

	var toAdd []string
	for _, entry := range gitignoreBaseline {
		if !existing[entry] {
			toAdd = append(toAdd, entry)
		}
	}
	if len(toAdd) == 0 {
		return nil // already up to date
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open .gitignore: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "\n# forge — auto-generated baseline\n"); err != nil {
		return err
	}
	for _, entry := range toAdd {
		if _, err := fmt.Fprintln(f, entry); err != nil {
			return err
		}
	}
	fmt.Printf("forge hygiene: added %d baseline entries to .gitignore\n", len(toAdd))
	return nil
}
