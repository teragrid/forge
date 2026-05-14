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
// Package cmdclean implements `forge clean` (DEV-M0-15).
//
// Walks the project root, classifies each tracked path against the
// .forge/manifest scratch + managed pattern lists, and either reports
// (default / --check) or deletes (--apply) the candidates.
package cmdclean

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/manifest"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 1300..1399).
var (
	ErrCleanFound    = errcode.Register(errcode.Code(1300), "clean found unmanaged scratch files")
	ErrCleanFailed   = errcode.Register(errcode.Code(1301), "clean apply failed")
	ErrSecretTracked = errcode.Register(errcode.Code(1302), "tracked secret file detected")
)

// secretPatterns are basename patterns that indicate a potentially dangerous
// tracked file. Patterns use filepath.Match syntax.
var secretPatterns = []string{
	".env",
	".env.local",
	".env.*.local",
	"*.pem",
	"*.key",
	"id_rsa",
	"id_ed25519",
	"id_ecdsa",
	"id_dsa",
	"secrets.json",
	"credentials.json",
	".netrc",
}

// Result is the summary of a clean run.
type Result struct {
	Root           string   `json:"root"`
	ManifestPath   string   `json:"manifest_path"`
	Mode           string   `json:"mode"` // "check" or "apply"
	Candidates     []string `json:"candidates"`
	Deleted        []string `json:"deleted,omitempty"`
	TrackedSecrets []string `json:"tracked_secrets,omitempty"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "clean",
		Summary: "Find and (optionally) remove LLM-scratch and other unmanaged files.",
		Inputs: []string{
			"--check (default; report only, exit 1 if anything found)",
			"--apply (delete the candidates)",
			"--root <path> (project root; default cwd)",
			"--json (machine-readable output)",
		},
		Outputs:      []string{"stdout: candidate list (text or JSON)"},
		SideEffects:  []string{"--apply deletes files matching the manifest [scratch] section"},
		GatesTouched: []string{"§16.5.4 #11 — repo hygiene"},
		ErrorCodes:   []errcode.Code{ErrCleanFound, ErrCleanFailed, ErrSecretTracked},
		OutputFields: []string{"root", "mode", "candidates"},
	})
}

// New returns the cobra command.
func New() *cobra.Command {
	var (
		root   string
		check  bool
		apply  bool
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Find/remove unmanaged scratch files.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if apply && check {
				return errcode.New(ErrCleanFailed, "use either --check or --apply, not both", nil)
			}
			if !apply {
				check = true // default
			}
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrCleanFailed, "getwd", err)
				}
				root = cwd
			}

			res, err := Run(root, apply)
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(res); err != nil {
					return err
				}
			} else {
				renderText(cmd, res)
			}

			if len(res.TrackedSecrets) > 0 {
				return errcode.Newf(ErrSecretTracked, nil,
					"%d secret file(s) tracked by git; remove from index with 'git rm --cached'",
					len(res.TrackedSecrets))
			}
			if !apply && len(res.Candidates) > 0 {
				return errcode.Newf(ErrCleanFound, nil,
					"%d candidate(s) found; rerun with --apply to delete", len(res.Candidates))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().BoolVar(&check, "check", false, "report only (default)")
	cmd.Flags().BoolVar(&apply, "apply", false, "delete found candidates")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

// Run scans root and (optionally) deletes scratch candidates. Exposed for
// tests + future ship-checkpoint integration.
func Run(root string, apply bool) (*Result, error) {
	mfPath := filepath.Join(root, manifest.DefaultPath)
	mf, err := manifest.Load(mfPath)
	if err != nil {
		return nil, err
	}

	mode := "check"
	if apply {
		mode = "apply"
	}
	res := &Result{Root: root, ManifestPath: mf.Path, Mode: mode}

	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if p == root {
			return nil
		}
		// Always skip .git for safety.
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if mf.IsScratch(rel) {
			res.Candidates = append(res.Candidates, rel)
			if d.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	if walkErr != nil {
		return nil, errcode.New(ErrCleanFailed, "walk", walkErr)
	}
	sort.Strings(res.Candidates)

	// Secret guard: detect files that should never be tracked by git.
	// Managed files (e.g. test fixtures) are excluded from this check.
	secrets, err := checkTrackedSecrets(root, mf)
	if err == nil {
		res.TrackedSecrets = secrets
	}
	// err != nil means git is unavailable — skip silently (graceful degradation).

	if apply {
		for _, c := range res.Candidates {
			full := filepath.Join(root, c)
			if err := os.RemoveAll(full); err != nil {
				return nil, errcode.Newf(ErrCleanFailed, err, "remove %s", c)
			}
			res.Deleted = append(res.Deleted, c)
		}
	}
	return res, nil
}

func renderText(cmd *cobra.Command, r *Result) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "forge clean (%s) root=%s\n", r.Mode, r.Root)
	fmt.Fprintf(w, "manifest: %s\n", r.ManifestPath)
	if len(r.TrackedSecrets) > 0 {
		fmt.Fprintf(w, "\nWARNING: %d secret file(s) tracked by git:\n", len(r.TrackedSecrets))
		for _, s := range r.TrackedSecrets {
			fmt.Fprintf(w, "  secret  %s\n", s)
		}
		fmt.Fprintln(w, "  → remove from index: git rm --cached <file>")
	}
	if len(r.Candidates) == 0 {
		fmt.Fprintln(w, "no candidates found.")
		return
	}
	fmt.Fprintf(w, "%d candidate(s):\n", len(r.Candidates))
	for _, c := range r.Candidates {
		marker := "would delete"
		if r.Mode == "apply" {
			marker = "deleted    "
		}
		fmt.Fprintf(w, "  %s  %s\n", marker, c)
	}
	if !strings.EqualFold(r.Mode, "apply") {
		fmt.Fprintln(w, "\nrerun with --apply to delete.")
	}
}

// checkTrackedSecrets runs `git ls-files` in root and returns any filenames
// that match secretPatterns. Files matching managed manifest patterns (e.g.
// test fixtures) are excluded. Returns (nil, non-nil) if git is unavailable
// (caller skips the check silently for graceful degradation).
func checkTrackedSecrets(root string, mf manifest.File) ([]string, error) {
	cmd := exec.Command("git", "-C", root, "ls-files")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	var found []string
	for _, line := range strings.Split(out.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip files that are explicitly managed (e.g. test fixtures).
		if manifest.Match(mf.Managed, filepath.ToSlash(line)) {
			continue
		}
		base := filepath.Base(filepath.FromSlash(line))
		for _, pat := range secretPatterns {
			if ok, _ := filepath.Match(pat, base); ok {
				found = append(found, line)
				break
			}
		}
	}
	sort.Strings(found)
	return found, nil
}
