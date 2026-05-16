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

// G-047: Embedding deduplication — content-addressed embedding store.
// Identical chunks are embedded once across the workspace; the dedup ratio
// is tracked and reported in `forge insights`.
package llmcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// EmbeddingVector is a dense float32 embedding.
type EmbeddingVector []float32

// EmbeddingRecord is one entry in the content-addressed store.
type EmbeddingRecord struct {
	ContentHash string          `json:"content_hash"`
	Model       string          `json:"model"`
	Vector      EmbeddingVector `json:"vector"`
}

// EmbeddingStore is a content-addressed embedding cache stored under
// <root>/.forge/cache/embeddings/.
type EmbeddingStore struct {
	mu     sync.RWMutex
	dir    string
	hits   int64
	misses int64
}

// OpenEmbeddingStore opens (or creates) the embedding store directory.
func OpenEmbeddingStore(root string) (*EmbeddingStore, error) {
	dir := filepath.Join(root, ".forge", "cache", "embeddings")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("embedding store: mkdir %s: %w", dir, err)
	}
	return &EmbeddingStore{dir: dir}, nil
}

// Get returns the cached embedding for (content, model), or nil if not cached.
func (s *EmbeddingStore) Get(content, model string) *EmbeddingRecord {
	key := contentKey(content, model)
	path := filepath.Join(s.dir, key+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		s.mu.Lock()
		s.misses++
		s.mu.Unlock()
		return nil
	}
	var rec EmbeddingRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		s.mu.Lock()
		s.misses++
		s.mu.Unlock()
		return nil
	}
	s.mu.Lock()
	s.hits++
	s.mu.Unlock()
	return &rec
}

// Put stores an embedding for (content, model).
func (s *EmbeddingStore) Put(content, model string, vector EmbeddingVector) error {
	key := contentKey(content, model)
	rec := EmbeddingRecord{
		ContentHash: key,
		Model:       model,
		Vector:      vector,
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("embedding store put: marshal: %w", err)
	}
	path := filepath.Join(s.dir, key+".json")
	return os.WriteFile(path, data, 0o600)
}

// Stats returns deduplication statistics: (hits, misses, dedupRatio).
func (s *EmbeddingStore) Stats() (hits, misses int64, ratio float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, m := s.hits, s.misses
	total := h + m
	if total == 0 {
		return h, m, 0
	}
	return h, m, float64(h) / float64(total)
}

func contentKey(content, model string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s", model, content)
	return hex.EncodeToString(h.Sum(nil))[:24]
}
