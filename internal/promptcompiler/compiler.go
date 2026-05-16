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

// Package promptcompiler implements G-040: prompt template compilation.
//
// Compile() strips comment lines (lines beginning with "//--"), collapses
// consecutive blank lines, and trims leading/trailing whitespace so that
// prompts rendered from .forge/prompts/*.prompt.md files do not waste tokens
// on developer annotations.
package promptcompiler

import (
	"strings"
)

// Compile processes a raw prompt template string and returns a cleaned,
// token-efficient version:
//   - Lines starting with "//--" (forge prompt comments) are removed.
//   - Multiple consecutive blank lines are collapsed to one.
//   - Leading and trailing whitespace is trimmed.
func Compile(src string) string {
	lines := strings.Split(src, "\n")
	var out []string
	blank := 0
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		// Strip forge prompt-comment lines.
		if strings.HasPrefix(trimmed, "//--") {
			continue
		}
		if trimmed == "" {
			blank++
			if blank <= 1 {
				out = append(out, "")
			}
			continue
		}
		blank = 0
		out = append(out, l)
	}
	// Trim leading/trailing blank lines.
	for len(out) > 0 && strings.TrimSpace(out[0]) == "" {
		out = out[1:]
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}
