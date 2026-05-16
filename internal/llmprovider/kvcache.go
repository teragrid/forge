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

// Package llmprovider — G-043: KV-cache prefix-break detection.
//
// Many LLM providers implement server-side KV-caching for prompt prefixes.
// When the system-prompt or a large context block changes, the prefix cache is
// invalidated, causing a measurable latency spike and extra compute cost.
//
// WarnPrefixBreak detects this by comparing the current system-prompt hash
// against the last known hash stored in .forge/cache/kv-prefix.json. If the
// prefix has changed, it logs a warning so the caller can investigate.
package llmprovider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const kvPrefixFile = ".forge/cache/kv-prefix.json"

type kvPrefixRecord struct {
	SystemPromptHash string `json:"system_prompt_hash"`
}

// WarnPrefixBreak compares the SHA-256 hash of systemPrompt against the last
// recorded value in .forge/cache/kv-prefix.json. If the hash differs, a
// warning is printed and the new hash is persisted. This is a no-op (returns
// without warning) if no prior record exists.
//
// root is the project root directory. Call this before every LLM Complete().
func WarnPrefixBreak(root, systemPrompt string) {
	h := sha256.Sum256([]byte(systemPrompt))
	currentHash := hex.EncodeToString(h[:])

	path := filepath.Join(root, kvPrefixFile)
	data, err := os.ReadFile(path)
	if err == nil {
		var rec kvPrefixRecord
		if json.Unmarshal(data, &rec) == nil && rec.SystemPromptHash != currentHash {
			fmt.Fprintf(os.Stderr,
				"forge warn: system-prompt prefix changed — LLM KV-cache will be invalidated "+
					"(prev=%s curr=%s). Consider pinning the system-prompt prefix.\n",
				rec.SystemPromptHash[:8], currentHash[:8])
		}
	}

	// Persist current hash.
	rec := kvPrefixRecord{SystemPromptHash: currentHash}
	out, _ := json.Marshal(rec)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, out, 0o600)
}
