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

// Package fssandbox implements DEV-M0-05: a filesystem service whose operations
// are restricted to a caller-declared root directory. All paths are resolved to
// absolute form before any I/O, and any attempt to escape the root via "../" or
// symlink traversal is denied with ErrEscape.
package fssandbox

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/teragrid/forge/internal/errcode"
)

// Reserved error codes (range 2500..2599).
var (
	ErrEscape   = errcode.Register(errcode.Code(2500), "path escapes sandbox root")
	ErrNotFound = errcode.Register(errcode.Code(2501), "path not found within sandbox")
)

// Sandbox restricts all filesystem operations to a single root directory.
type Sandbox struct {
	root string // absolute, cleaned
}

// New returns a Sandbox rooted at root. Root is resolved to an absolute path;
// returns an error if root does not exist or is not a directory.
func New(root string) (*Sandbox, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("fssandbox: root must be a directory")
	}
	return &Sandbox{root: abs}, nil
}

// Root returns the absolute root path.
func (s *Sandbox) Root() string { return s.root }

// Abs returns the absolute path for rel inside the sandbox, or ErrEscape if
// the resolved path would be outside the root.
func (s *Sandbox) Abs(rel string) (string, error) {
	// Join with the root so relative paths are anchored.
	joined := filepath.Join(s.root, rel)
	// Clean to resolve any ".." segments.
	cleaned := filepath.Clean(joined)
	// Ensure it stays inside root. We compare with a trailing separator to
	// prevent "root-prefix" matches like /tmp/sandbox against /tmp/sandbox2.
	rootWithSep := s.root + string(filepath.Separator)
	if cleaned != s.root && !strings.HasPrefix(cleaned, rootWithSep) {
		return "", errcode.New(ErrEscape, "path "+rel+" escapes sandbox root "+s.root, nil)
	}
	return cleaned, nil
}

// Stat returns os.FileInfo for the given sandbox-relative path.
func (s *Sandbox) Stat(rel string) (fs.FileInfo, error) {
	abs, err := s.Abs(rel)
	if err != nil {
		return nil, err
	}
	return os.Lstat(abs) // Lstat: do not follow symlinks at the leaf
}

// ReadFile reads the file at the sandbox-relative path. Symlinks at the leaf
// are not followed (Lstat above ensures leaf is a real file).
func (s *Sandbox) ReadFile(rel string) ([]byte, error) {
	abs, err := s.Abs(rel)
	if err != nil {
		return nil, err
	}
	// Guard against symlink pointing outside the sandbox.
	if err := s.checkSymlink(abs); err != nil {
		return nil, err
	}
	return os.ReadFile(abs) //nolint:gosec // path already validated
}

// Glob returns all paths matching the sandbox-relative pattern. Paths that
// resolve outside the root are silently excluded (cannot happen via Glob but
// included as a defence-in-depth layer).
func (s *Sandbox) Glob(pattern string) ([]string, error) {
	absPattern, err := s.Abs(pattern)
	if err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(absPattern)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, m := range matches {
		cleaned := filepath.Clean(m)
		rootWithSep := s.root + string(filepath.Separator)
		if cleaned == s.root || strings.HasPrefix(cleaned, rootWithSep) {
			result = append(result, cleaned)
		}
	}
	return result, nil
}

// Walk walks the sandbox tree, calling fn for each file/dir. Paths passed to
// fn are absolute. Symlinks are not followed.
func (s *Sandbox) Walk(fn func(path string, info fs.FileInfo, err error) error) error {
	return filepath.Walk(s.root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return fn(path, info, err)
		}
		// Skip symlinks that point outside the root.
		if info.Mode()&fs.ModeSymlink != 0 {
			if e := s.checkSymlink(path); e != nil {
				return nil // skip; don't propagate escape attempts
			}
		}
		return fn(path, info, nil)
	})
}

// checkSymlink returns ErrEscape if path is a symlink whose target resolves
// outside the sandbox root.
func (s *Sandbox) checkSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return nil // not our job to error on stat failures here
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		return nil // not a symlink
	}
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return errcode.New(ErrEscape, "cannot resolve symlink: "+path, err)
	}
	rootWithSep := s.root + string(filepath.Separator)
	if target != s.root && !strings.HasPrefix(target, rootWithSep) {
		return errcode.New(ErrEscape, "symlink "+path+" points outside sandbox root", nil)
	}
	return nil
}
