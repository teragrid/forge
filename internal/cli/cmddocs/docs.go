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

// Package cmddocs implements `forge docs` (spec §4).
//
// Synchronises and heals project documentation from code and ADRs.
// Sub-commands: sync, heal.
package cmddocs

import (
	"bufio"
	"fmt"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 5000..5099).
var (
	ErrDocsFailed = errcode.Register(errcode.Code(5000), "docs operation failed")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "docs",
		Summary: "Synchronise and heal project documentation from code and ADRs (spec §4).",
		Inputs: []string{
			"sync  — regenerate docs from code comments and ADRs",
			"heal  — fix broken internal links and stale section headers",
			"--root <path>",
			"--dry-run",
		},
		Outputs:      []string{"stdout: sync/heal report"},
		SideEffects:  []string{"sync/heal: writes to docs/ directory"},
		GatesTouched: []string{"§4 docs"},
		ErrorCodes:   []errcode.Code{ErrDocsFailed},
	})
}

// New returns the cobra command for `forge docs`.
func New() *cobra.Command {
	var (
		root   string
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:   "docs <sync|heal>",
		Short: "Sync and heal project documentation (M2).",
		Long: "forge docs manages project documentation derived from code and ADRs.\n\n" +
			"Sub-commands:\n" +
			"  sync  — regenerate docs from code comments, error codes, and ADRs\n" +
			"  heal  — fix broken internal links and stale section headers",
	}
	cmd.PersistentFlags().StringVar(&root, "root", "", "Project root (default: cwd)")
	cmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing")
	cmd.AddCommand(
		newSyncCmd(&root, &dryRun),
		newHealCmd(&root, &dryRun),
	)
	return cmd
}

func newSyncCmd(root *string, dryRun *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Regenerate docs from code comments, error codes, and ADRs.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveRoot(*root)
			if err != nil {
				return errcode.New(ErrDocsFailed, "resolve root", err)
			}
			out := cmd.OutOrStdout()
			// 1. Scan Go packages for exported types/functions
			pkgDocs, err := scanGoPkgDocs(filepath.Join(r, "internal"))
			if err != nil {
				fmt.Fprintf(out, "docs sync: go scan warning: %v\n", err)
			}
			// 2. Build API reference markdown
			apiMd := buildAPIRef(pkgDocs)
			apiPath := filepath.Join(r, "docs", "API_REFERENCE.md")
			if *dryRun {
				fmt.Fprintf(out, "docs sync (dry-run): would write %s (%d bytes)\n", relOrAbs(r, apiPath), len(apiMd))
			} else {
				if err := os.WriteFile(apiPath, []byte(apiMd), 0o644); err != nil {
					return errcode.New(ErrDocsFailed, "write API_REFERENCE.md", err)
				}
				fmt.Fprintf(out, "docs sync: wrote %s\n", relOrAbs(r, apiPath))
			}
			// 3. List ADRs and regenerate ADR index
			adrDir := filepath.Join(r, "docs", "adr")
			adrIndex, err := buildADRIndex(adrDir)
			if err != nil {
				fmt.Fprintf(out, "docs sync: ADR scan warning: %v\n", err)
			}
			indexPath := filepath.Join(adrDir, "README.md")
			if *dryRun {
				fmt.Fprintf(out, "docs sync (dry-run): would update %s (%d entries)\n", relOrAbs(r, indexPath), len(adrIndex))
			} else {
				if err := writeADRIndex(indexPath, adrIndex); err != nil {
					fmt.Fprintf(out, "docs sync: ADR index warning: %v\n", err)
				} else {
					fmt.Fprintf(out, "docs sync: updated %s (%d entries)\n", relOrAbs(r, indexPath), len(adrIndex))
				}
			}
			fmt.Fprintln(out, "docs sync: complete")
			return nil
		},
	}
}

func newHealCmd(root *string, dryRun *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "heal",
		Short: "Fix broken internal links and stale section headers.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveRoot(*root)
			if err != nil {
				return errcode.New(ErrDocsFailed, "resolve root", err)
			}
			out := cmd.OutOrStdout()
			docsDir := filepath.Join(r, "docs")
			broken, err := findBrokenLinks(r, docsDir)
			if err != nil {
				return errcode.New(ErrDocsFailed, "scan links", err)
			}
			if len(broken) == 0 {
				fmt.Fprintln(out, "docs heal: no broken links found")
				return nil
			}
			for _, b := range broken {
				fmt.Fprintf(out, "broken link: %s: %s\n", relOrAbs(r, b.file), b.link)
			}
			if *dryRun {
				fmt.Fprintf(out, "docs heal (dry-run): %d broken links — re-run without --dry-run to remove stale links\n", len(broken))
			} else {
				fixed, err := removeStaleLinks(broken)
				if err != nil {
					return errcode.New(ErrDocsFailed, "remove stale links", err)
				}
				fmt.Fprintf(out, "docs heal: removed %d stale links\n", fixed)
			}
			return nil
		},
	}
}

// --- helpers ---

type pkgDocEntry struct {
	pkg   string
	path  string
	funcs []string
	types []string
}

func scanGoPkgDocs(dir string) ([]pkgDocEntry, error) {
	var entries []pkgDocEntry
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil // skip unparseable dirs
		}
		for name, pkg := range pkgs {
			if strings.HasSuffix(name, "_test") {
				continue
			}
			d := doc.New(pkg, path, 0)
			entry := pkgDocEntry{pkg: name, path: path}
			for _, f := range d.Funcs {
				entry.funcs = append(entry.funcs, f.Name)
			}
			for _, t := range d.Types {
				entry.types = append(entry.types, t.Name)
			}
			entries = append(entries, entry)
		}
		return nil
	})
	return entries, err
}

func buildAPIRef(entries []pkgDocEntry) string {
	var b strings.Builder
	b.WriteString("# API Reference\n\n")
	b.WriteString("*Auto-generated by `forge docs sync`. Do not edit manually.*\n\n")
	for _, e := range entries {
		if len(e.funcs)+len(e.types) == 0 {
			continue
		}
		b.WriteString("## `" + e.pkg + "`\n\n")
		if len(e.types) > 0 {
			b.WriteString("**Types:** " + strings.Join(e.types, ", ") + "\n\n")
		}
		if len(e.funcs) > 0 {
			b.WriteString("**Functions:** " + strings.Join(e.funcs, ", ") + "\n\n")
		}
	}
	return b.String()
}

type adrEntry struct{ num, title, file string }

var adrHeaderRe = regexp.MustCompile(`(?m)^#\s+ADR-(\d+)[:\s]+(.+)$`)

func buildADRIndex(dir string) ([]adrEntry, error) {
	var entries []adrEntry
	files, err := filepath.Glob(filepath.Join(dir, "ADR-*.md"))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		m := adrHeaderRe.FindSubmatch(data)
		if m == nil {
			continue
		}
		entries = append(entries, adrEntry{
			num:   string(m[1]),
			title: strings.TrimSpace(string(m[2])),
			file:  filepath.Base(f),
		})
	}
	return entries, nil
}

func writeADRIndex(path string, entries []adrEntry) error {
	var b strings.Builder
	b.WriteString("# Architecture Decision Records\n\n")
	b.WriteString("*Auto-generated index.*\n\n")
	for _, e := range entries {
		b.WriteString(fmt.Sprintf("- [ADR-%s: %s](%s)\n", e.num, e.title, e.file))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

type brokenLink struct{ file, link string }

var mdLinkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

func findBrokenLinks(root, dir string) ([]brokenLink, error) {
	var result []brokenLink
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			for _, m := range mdLinkRe.FindAllStringSubmatch(line, -1) {
				link := m[2]
				if strings.HasPrefix(link, "http") || strings.HasPrefix(link, "#") {
					continue
				}
				// Resolve relative to the containing file's directory
				abs := filepath.Join(filepath.Dir(path), link)
				if _, err := os.Stat(abs); os.IsNotExist(err) {
					result = append(result, brokenLink{file: path, link: link})
				}
			}
		}
		return nil
	})
	return result, err
}

func removeStaleLinks(links []brokenLink) (int, error) {
	// Group by file
	byFile := map[string][]string{}
	for _, l := range links {
		byFile[l.file] = append(byFile[l.file], l.link)
	}
	var total int
	for file, stale := range byFile {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		content := string(data)
		for _, link := range stale {
			// Replace [text](link) with just "text"
			re := regexp.MustCompile(`\[([^\]]+)\]\(` + regexp.QuoteMeta(link) + `\)`)
			content = re.ReplaceAllString(content, "$1")
			total++
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			return total, err
		}
	}
	return total, nil
}

func relOrAbs(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}

func resolveRoot(root string) (string, error) {
	if root != "" {
		return root, nil
	}
	return os.Getwd()
}
