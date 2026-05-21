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

package scaffold

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/teragrid/forge/internal/errcode"
)

// Composition error codes (range 6450..6499).
var (
	ErrFileConflict   = errcode.Register(errcode.Code(6450), "file conflict during module composition")
	ErrModuleNotFound = errcode.Register(errcode.Code(6451), "module scaffold directory not found")
)

// FileConflictError describes a non-additive file collision.
type FileConflictError struct {
	Path    string
	Modules [2]string // the two module IDs that both provide Path
}

func (e *FileConflictError) Error() string {
	return fmt.Sprintf("[%s] file conflict: %q appears in both %q and %q",
		ErrFileConflict.Format(), e.Path, e.Modules[0], e.Modules[1])
}

// ModuleNotFoundError describes a missing module scaffold directory.
type ModuleNotFoundError struct {
	ID string
}

func (e *ModuleNotFoundError) Error() string {
	return fmt.Sprintf("[%s] module scaffold not found: %q", ErrModuleNotFound.Format(), e.ID)
}

// additivePaths are file paths (relative to module scaffold root) that are
// merged by concatenation + line-deduplication rather than by conflict detection.
var additivePaths = map[string]bool{
	".env.example": true,
	".gitignore":   true,
}

// CompositionOptions controls module composition behaviour.
type CompositionOptions struct {
	// ModulesRoot is the parent directory that contains module scaffold dirs.
	// Each module ID maps to <ModulesRoot>/<module-id>/scaffold/.
	// Defaults to "forge-knowledge/templates" relative to cwd.
	ModulesRoot string

	// Language, when set, causes Compose to prefer a language-specific scaffold
	// variant. For each module, if <ModulesRoot>/<module-id>/scaffold-<Language>/
	// exists it is used instead of the default scaffold/ directory.
	// Example: Language="python" will pick scaffold-python/ over scaffold/ for
	// modules that have a Python-specific scaffold.
	Language string

	// SkipMissing, when true, silently skips modules whose scaffold directory
	// does not exist instead of returning a ModuleNotFoundError. The IDs of
	// skipped modules are collected in ComposedResult.MissingModules.
	SkipMissing bool
}

// ComposedResult holds the composed in-memory file tree before it is written.
type ComposedResult struct {
	// Files maps relative path → file contents (bytes).
	Files map[string][]byte

	// MissingModules lists module IDs whose scaffold directory was not found.
	// Populated only when CompositionOptions.SkipMissing is true.
	MissingModules []string
}

// Compose merges the scaffold directories of the listed modules into an
// in-memory file tree. Additive files (.env.example, .gitignore) are
// concatenated and deduplicated; all other files must be unique across modules.
// The result is deterministic for the same input slice.
func Compose(modules []string, opts CompositionOptions) (*ComposedResult, error) {
	if opts.ModulesRoot == "" {
		opts.ModulesRoot = "forge-knowledge/templates"
	}

	result := &ComposedResult{Files: make(map[string][]byte)}
	// additiveLines accumulates lines per additive path for deduplication.
	additiveLines := make(map[string][]string)
	// seenInModule tracks which module first contributed each non-additive path.
	seenInModule := make(map[string]string)

	for _, modID := range modules {
		scaffoldDir := filepath.Join(opts.ModulesRoot, filepath.FromSlash(modID), "scaffold")
		// TG-14: prefer language-specific scaffold variant when available.
		if opts.Language != "" {
			langDir := filepath.Join(opts.ModulesRoot, filepath.FromSlash(modID), "scaffold-"+opts.Language)
			if _, statErr := os.Stat(langDir); statErr == nil {
				scaffoldDir = langDir
			}
		}
		if _, err := os.Stat(scaffoldDir); os.IsNotExist(err) {
			if opts.SkipMissing {
				result.MissingModules = append(result.MissingModules, modID)
				continue
			}
			return nil, &ModuleNotFoundError{ID: modID}
		}

		walkErr := filepath.WalkDir(scaffoldDir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(scaffoldDir, p)
			rel = filepath.ToSlash(rel) // normalise to forward slashes

			body, readErr := os.ReadFile(p) //nolint:gosec // walking trusted modules dir
			if readErr != nil {
				return readErr
			}

			if additivePaths[rel] {
				// Collect lines; dedup happens after all modules are walked.
				lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
				additiveLines[rel] = append(additiveLines[rel], lines...)
				return nil
			}

			if first, seen := seenInModule[rel]; seen {
				return &FileConflictError{Path: rel, Modules: [2]string{first, modID}}
			}
			seenInModule[rel] = modID
			result.Files[rel] = body
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}

	// Finalise additive files: deduplicate lines, preserve order.
	for rel, lines := range additiveLines {
		result.Files[rel] = []byte(deduplicateLines(lines))
	}

	return result, nil
}

// WriteComposed writes a ComposedResult into targetDir, creating directories
// as needed.
func WriteComposed(res *ComposedResult, targetDir string) ([]string, error) {
	// Sort paths for deterministic output.
	paths := make([]string, 0, len(res.Files))
	for p := range res.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var written []string
	for _, rel := range paths {
		dst := filepath.Join(targetDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { //nolint:gosec
			return nil, errcode.Newf(ErrWrite, err, "mkdir for %s", rel)
		}
		if err := os.WriteFile(dst, res.Files[rel], 0o600); err != nil {
			return nil, errcode.Newf(ErrWrite, err, "write %s", rel)
		}
		written = append(written, rel)
	}
	return written, nil
}

// deduplicateLines removes duplicate lines while preserving order.
// Empty trailing lines are normalised to a single trailing newline.
func deduplicateLines(lines []string) string {
	seen := make(map[string]bool, len(lines))
	var out []string
	for _, l := range lines {
		if l == "" {
			continue // strip empties; we'll add one trailing newline at end
		}
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}
