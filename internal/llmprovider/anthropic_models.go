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

// anthropic_models.go — live model discovery + disk cache for the Anthropic
// provider (dynamic-fault-tolerant-model-selection AC5/AC6), mirroring
// CopilotProvider.loadModels (copilot.go:106-152).
//
// Discovery results are cached under .forge/cache/models/anthropic.json with
// a 24h TTL, following the existing .forge/cache/ convention
// (llmcache.DefaultDir = ".forge/cache/llm", kvPrefixFile =
// ".forge/cache/kv-prefix.json"). When the cache is warm (< TTL old) no
// network call is made at all, so a normal checkpoint run pays zero extra
// latency. When discovery is unreachable, the last-known-good cache is used
// even if stale; only when no cache has ever been written does Capabilities()
// fall back to the hardcoded anthropicKnownModels list — discovery is a
// quality-of-life improvement, never a hard dependency (AC6).
package llmprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// anthropicModelCacheFile is the on-disk cache path (relative to the process
// working directory, which is the project root by the same convention
// detectAnthropicKey/Detect already rely on — Provider construction carries
// no explicit root parameter).
const anthropicModelCacheFile = ".forge/cache/models/anthropic.json"

// anthropicModelCacheTTL is how long a discovered model list is considered
// fresh before a re-fetch is attempted (proposed in spec Open Questions; 24h
// by analogy with typical model-catalog change frequency).
const anthropicModelCacheTTL = 24 * time.Hour

// anthropicModelCache is the on-disk cache payload.
type anthropicModelCache struct {
	Models    []string  `json:"models"`
	FetchedAt time.Time `json:"fetched_at"`
}

// loadModels populates a.cachedModels from (in priority order): a fresh
// on-disk cache (no network call), a live call to the Anthropic /v1/models
// endpoint (which refreshes the cache on success), or a stale on-disk cache
// as a last resort when discovery fails. Called lazily once from
// Capabilities() via sync.Once.
func (a *AnthropicAdapter) loadModels() {
	cachePath := anthropicCachePath()
	cached, fetchedAt, ok := readAnthropicModelCache(cachePath)
	if ok && time.Since(fetchedAt) < anthropicModelCacheTTL {
		a.cachedModels = cached
		return
	}

	live, err := a.fetchLiveModels()
	if err == nil && len(live) > 0 {
		a.cachedModels = live
		_ = writeAnthropicModelCache(cachePath, live)
		return
	}

	// Discovery unreachable or failed — fall back to the stale cache if one
	// exists (AC6); Capabilities() falls back further to anthropicKnownModels
	// when a.cachedModels remains empty.
	if len(cached) > 0 {
		a.cachedModels = cached
	}
}

// anthropicCachePath resolves the on-disk cache location relative to the
// process working directory.
func anthropicCachePath() string {
	root, err := os.Getwd()
	if err != nil {
		root = "."
	}
	return filepath.Join(root, anthropicModelCacheFile)
}

// readAnthropicModelCache reads and parses the cache file. ok is false when
// the file is absent, unreadable, or empty.
func readAnthropicModelCache(path string) (models []string, fetchedAt time.Time, ok bool) {
	data, err := os.ReadFile(path) //nolint:gosec // path is derived from cwd, not user input
	if err != nil {
		return nil, time.Time{}, false
	}
	var c anthropicModelCache
	if err := json.Unmarshal(data, &c); err != nil || len(c.Models) == 0 {
		return nil, time.Time{}, false
	}
	return c.Models, c.FetchedAt, true
}

// writeAnthropicModelCache persists a freshly discovered model list.
func writeAnthropicModelCache(path string, models []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("anthropic: create cache dir: %w", err)
	}
	data, err := json.MarshalIndent(anthropicModelCache{
		Models:    models,
		FetchedAt: time.Now().UTC(),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("anthropic: marshal model cache: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// fetchLiveModels calls GET {baseURL}/models to discover currently available
// models. Bounded to a 5s timeout — the same bound CopilotProvider.loadModels
// uses — so discovery can never add meaningful latency to a checkpoint run.
func (a *AnthropicAdapter) fetchLiveModels() ([]string, error) {
	if a.apiKey == "" {
		return nil, fmt.Errorf("anthropic: no API key for model discovery")
	}
	baseURL := a.baseURL
	if baseURL == "" {
		baseURL = anthropicAPIBase
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)
	req.Header.Set("Accept", "application/json")

	client := a.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic: models endpoint returned HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16)) // 64 KB max
	if err != nil {
		return nil, err
	}

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(body.Data))
	for _, m := range body.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("anthropic: models endpoint returned no models")
	}
	return models, nil
}
