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

// Package storage provides an S3-compatible storage abstraction (M2-07).
//
// The Adapter interface is intentionally minimal: Put, Get, Delete, List.
// The LocalAdapter stores objects on the local filesystem (useful for
// development and testing). A real S3-compatible adapter can be substituted
// at init time.
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Object is a lightweight descriptor for a stored object.
type Object struct {
	Key  string
	Size int64
}

// Adapter is the storage abstraction.
type Adapter interface {
	// Put writes data under key.
	Put(ctx context.Context, key string, r io.Reader) error
	// Get retrieves data stored at key.
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete removes the object at key.
	Delete(ctx context.Context, key string) error
	// List returns all objects whose keys share the given prefix.
	List(ctx context.Context, prefix string) ([]Object, error)
	// Name returns the adapter identifier.
	Name() string
}

// ── Local filesystem adapter ─────────────────────────────────────────────────

// LocalAdapter stores objects as files under a base directory.
// Keys are treated as relative paths; path traversal is rejected.
type LocalAdapter struct {
	base string
}

// NewLocalAdapter creates a LocalAdapter rooted at base.
// The directory is created if it does not exist.
func NewLocalAdapter(base string) (*LocalAdapter, error) {
	if err := os.MkdirAll(base, 0o700); err != nil {
		return nil, fmt.Errorf("storage/local: mkdir %s: %w", base, err)
	}
	abs, err := filepath.Abs(base)
	if err != nil {
		return nil, err
	}
	return &LocalAdapter{base: abs}, nil
}

func (a *LocalAdapter) Name() string { return "local" }

// resolvePath resolves key to an absolute path inside a.base.
// It rejects keys that would escape the base directory (path traversal).
func (a *LocalAdapter) resolvePath(key string) (string, error) {
	// Sanitize: reject empty, absolute, or directory-traversal keys.
	if key == "" {
		return "", fmt.Errorf("storage/local: key must not be empty")
	}
	clean := filepath.Clean(key)
	abs := filepath.Join(a.base, clean)
	// Verify the resolved path stays within base.
	if !strings.HasPrefix(abs, a.base+string(filepath.Separator)) && abs != a.base {
		return "", fmt.Errorf("storage/local: key %q escapes base directory", key)
	}
	return abs, nil
}

// Put writes r to key, creating intermediate directories as needed.
func (a *LocalAdapter) Put(_ context.Context, key string, r io.Reader) error {
	path, err := a.resolvePath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("storage/local: put mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("storage/local: put open: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("storage/local: put write: %w", err)
	}
	return nil
}

// Get opens the file at key for reading.
func (a *LocalAdapter) Get(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := a.resolvePath(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("storage/local: get %q: %w", key, err)
	}
	return f, nil
}

// Delete removes the file at key. Returns nil if the file does not exist.
func (a *LocalAdapter) Delete(_ context.Context, key string) error {
	path, err := a.resolvePath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storage/local: delete %q: %w", key, err)
	}
	return nil
}

// List returns all objects whose resolved paths share the given prefix.
func (a *LocalAdapter) List(_ context.Context, prefix string) ([]Object, error) {
	var objs []Object
	basePrefix := filepath.Join(a.base, prefix)
	err := filepath.WalkDir(a.base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasPrefix(path, basePrefix) {
			return nil
		}
		rel, err := filepath.Rel(a.base, path)
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		objs = append(objs, Object{Key: rel, Size: info.Size()})
		return nil
	})
	return objs, err
}

// ── Default adapter ──────────────────────────────────────────────────────────

var defaultAdapter Adapter

// Default returns the process-wide default storage adapter.
// Falls back to a LocalAdapter rooted at os.TempDir()/forge-storage if none
// has been set via SetDefault.
func Default() Adapter {
	if defaultAdapter != nil {
		return defaultAdapter
	}
	a, err := NewLocalAdapter(filepath.Join(os.TempDir(), "forge-storage"))
	if err != nil {
		panic("storage: could not initialise default local adapter: " + err.Error())
	}
	defaultAdapter = a
	return defaultAdapter
}

// SetDefault replaces the process-wide default storage adapter.
func SetDefault(a Adapter) {
	defaultAdapter = a
}
