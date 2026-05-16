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

// G-072: Privacy invariant for LLM context bundles.
//
// quarantinedDirs and quarantinedExtensions define paths and file types that
// MUST NEVER be included in any LLM context bundle, log, or telemetry payload.
//
// Rules:
//  1. .forge/llm-scratch/ — raw LLM scratch output; may contain secrets
//  2. .forge/trash/ — deleted/archived content; may contain PII
//  3. .forge/outbox/ — mutation events; may contain PII
//  4. .forge/session/ — session logs; may contain runtime secrets
//  5. .forge/cache/ — KV cache; may contain raw prompts
//
// IsPrivatePath returns true for any path that must not be sent to an LLM.
// All context-bundle generators MUST call IsPrivatePath before including a
// file in a bundle.
package cmdcontext

import (
	"path/filepath"
	"strings"
)

// quarantinedDirs are .forge sub-directories whose contents must never enter
// any LLM context bundle, telemetry stream, or log line (G-072).
var quarantinedDirs = []string{
	filepath.Join(".forge", "llm-scratch"),
	filepath.Join(".forge", "trash"),
	filepath.Join(".forge", "outbox"),
	filepath.Join(".forge", "session"),
	filepath.Join(".forge", "cache"),
	filepath.Join(".forge", "scan-history"),
}

// IsPrivatePath reports whether path (relative to project root) must be
// excluded from LLM context bundles per the G-072 privacy invariant.
func IsPrivatePath(rel string) bool {
	rel = filepath.ToSlash(filepath.Clean(rel))
	for _, dir := range quarantinedDirs {
		prefix := filepath.ToSlash(dir) + "/"
		if strings.HasPrefix(rel, prefix) || rel == filepath.ToSlash(dir) {
			return true
		}
	}
	return false
}
