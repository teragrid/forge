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

// Package llmcache implements DEV-M1-07: a file-backed semantic cache for LLM
// responses, keyed by a SHA-256 hash of the (model, system-prompt, user-prompt)
// tuple.
//
// Cache hit: zero provider calls; stale on file change in the source path set.
// Cache miss: caller invokes the provider, then Stores the response.
//
// On-disk layout (under <root>/.forge/cache/llm/):
//
//	<sha256hex>.json  — one file per entry, containing CacheEntry JSON.
package llmcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultDir is the project-relative path for the LLM cache directory.
const DefaultDir = ".forge/cache/llm"

// CacheEntry is persisted on disk.
type CacheEntry struct {
	// Key is the hex-encoded SHA-256 of the request tuple.
	Key string `json:"key"`

	// Model is the LLM model that produced this response.
	Model string `json:"model"`

	// Response is the cached completion text.
	Response string `json:"response"`

	// CreatedAt is when this entry was written.
	CreatedAt time.Time `json:"created_at"`

	// SourcePaths are the file paths whose content was included in the prompt.
	// The cache is invalidated when any of these files changes.
	SourcePaths []string `json:"source_paths,omitempty"`

	// SourceHash is a combined SHA-256 of all SourcePaths' contents at write
	// time. A mismatch means at least one source file has changed.
	SourceHash string `json:"source_hash,omitempty"`
}

// Cache is a file-backed semantic response cache.
type Cache struct {
	dir string
}

// Open opens (or creates) the LLM cache rooted at dir.
func Open(dir string) (*Cache, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("llmcache: create cache dir %s: %w", dir, err)
	}
	return &Cache{dir: dir}, nil
}

// OpenDefault opens the default cache directory under root.
func OpenDefault(root string) (*Cache, error) {
	return Open(filepath.Join(root, DefaultDir))
}

// Key computes the cache key for a (model, systemPrompt, userPrompt) tuple.
func Key(model, systemPrompt, userPrompt string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s", model, systemPrompt, userPrompt)
	return hex.EncodeToString(h.Sum(nil))
}

// sourceHash returns a combined SHA-256 over the contents of paths.
// Missing files contribute an empty byte sequence (not an error).
func sourceHash(paths []string) string {
	h := sha256.New()
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			// Missing file → treat as empty; caller must re-hash on file deletion.
			continue
		}
		fmt.Fprintf(h, "%s\x00", p)
		h.Write(data)
		h.Write([]byte("\x01"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Get retrieves a cached response. Returns (entry, true) on a valid hit, or
// (nil, false) on a miss (entry absent or source files changed).
func (c *Cache) Get(key string, sourcePaths []string) (*CacheEntry, bool) {
	path := filepath.Join(c.dir, key+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var e CacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, false
	}
	// Validate source-file freshness.
	if len(sourcePaths) > 0 && e.SourceHash != "" {
		if sourceHash(sourcePaths) != e.SourceHash {
			return nil, false // source changed → cache stale
		}
	}
	return &e, true
}

// Store writes a response to the cache.
func (c *Cache) Store(key, model, response string, sourcePaths []string) error {
	e := CacheEntry{
		Key:         key,
		Model:       model,
		Response:    response,
		CreatedAt:   time.Now().UTC(),
		SourcePaths: sourcePaths,
	}
	if len(sourcePaths) > 0 {
		e.SourceHash = sourceHash(sourcePaths)
	}
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Errorf("llmcache: marshal entry: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(c.dir, key+".json")
	return os.WriteFile(path, data, 0o600)
}

// Delete removes a single cache entry.
func (c *Cache) Delete(key string) error {
	path := filepath.Join(c.dir, key+".json")
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Purge removes all entries from the cache directory.
func (c *Cache) Purge() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("llmcache: read dir: %w", err)
	}
	for _, de := range entries {
		if de.IsDir() || filepath.Ext(de.Name()) != ".json" {
			continue
		}
		if err := os.Remove(filepath.Join(c.dir, de.Name())); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("llmcache: remove %s: %w", de.Name(), err)
		}
	}
	return nil
}

// Stats returns (hits, misses) counts from calling Get since process start.
// NOTE: counters are in-memory only; they reset on process restart.
var (
	hitCount  int64
	missCount int64
)

// GetWithStats wraps Get and increments the in-memory counters.
func (c *Cache) GetWithStats(key string, sourcePaths []string) (*CacheEntry, bool) {
	e, ok := c.Get(key, sourcePaths)
	if ok {
		hitCount++
	} else {
		missCount++
	}
	return e, ok
}

// HitCount returns the number of cache hits since process start.
func HitCount() int64 { return hitCount }

// MissCount returns the number of cache misses since process start.
func MissCount() int64 { return missCount }
