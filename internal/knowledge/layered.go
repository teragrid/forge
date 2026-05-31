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

// layered.go — P1: 3-layer knowledge base (project > user > embedded global).
//
// LoadLayered merges three knowledge-base layers in precedence order:
//
//  1. Project layer    — <root>/.forge/kb/*.md  (highest precedence)
//  2. User/org layer   — ~/.forge/kb/*.md
//  3. Global embedded  — compiled-in index via Load()
//
// Entries with the same ID in a higher-precedence layer silently shadow
// lower-precedence entries.  Any unavailable layer is skipped without error.
// Markdown files use optional YAML frontmatter (--- … ---) for metadata;
// the body becomes the Snippet field.

package knowledge

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadLayered returns a merged Index from the three KB layers.
// The returned Index has Version set from the global embedded index.
// All errors from individual layers are silently swallowed so that a
// missing ~/.forge/kb/ directory never crashes the pipeline.
func LoadLayered(root string) (*Index, error) {
	global, err := Load()
	if err != nil || global == nil {
		global = &Index{Version: "0"}
	}

	// Seed the merged map from the (lowest precedence) global layer.
	merged := make(map[string]Entry, len(global.Entries))
	for _, e := range global.Entries {
		merged[e.ID] = e
	}

	// Layer 2 (user/org): ~/.forge/kb/
	if home, homeErr := os.UserHomeDir(); homeErr == nil {
		loadMarkdownLayer(filepath.Join(home, ".forge", "kb"), merged)
	}

	// Layer 1 (project): <root>/.forge/kb/
	loadMarkdownLayer(filepath.Join(root, ".forge", "kb"), merged)

	out := make([]Entry, 0, len(merged))
	for _, e := range merged {
		out = append(out, e)
	}
	return &Index{Version: global.Version, Entries: out}, nil
}

// loadMarkdownLayer reads every *.md file in dir, parses it as a KB entry,
// and upserts the result into merged (overwriting any entry with the same ID).
// A missing or unreadable dir is silently ignored.
func loadMarkdownLayer(dir string, merged map[string]Entry) {
	infos, err := os.ReadDir(dir)
	if err != nil {
		return // directory missing or unreadable — silently skip
	}
	for _, info := range infos {
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			continue
		}
		if entry, ok := parseKBMarkdown(filepath.Join(dir, info.Name())); ok {
			merged[entry.ID] = entry
		}
	}
}

// parseKBMarkdown parses a single KB markdown file.
// The file may open with a YAML frontmatter block (--- … ---).
// Recognised frontmatter keys: id, category, intent, tags (comma-separated).
// The body (after optional frontmatter) becomes the Snippet.
// The filename stem is used as the fallback ID when no id key is present.
func parseKBMarkdown(path string) (Entry, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Entry{}, false
	}
	stem := strings.TrimSuffix(filepath.Base(path), ".md")
	entry := Entry{
		ID:   stem,
		Path: path,
	}
	content := string(data)

	const fence = "---\n"
	if strings.HasPrefix(content, fence) || strings.HasPrefix(content, "---\r\n") {
		// Find the closing fence (skip the opening 4 chars).
		rest := content[4:]
		end := strings.Index(rest, "\n---")
		if end >= 0 {
			fm := rest[:end]
			body := rest[end+4:]
			parseFrontmatter(fm, &entry)
			entry.Snippet = strings.TrimSpace(body)
		} else {
			entry.Snippet = strings.TrimSpace(content)
		}
	} else {
		entry.Snippet = strings.TrimSpace(content)
	}

	if entry.ID == "" {
		entry.ID = stem
	}
	return entry, true
}

// parseFrontmatter extracts recognised YAML scalar fields from a frontmatter
// block (already stripped of the --- delimiters).
func parseFrontmatter(fm string, e *Entry) {
	scanner := bufio.NewScanner(strings.NewReader(fm))
	for scanner.Scan() {
		line := scanner.Text()
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		val := strings.TrimSpace(line[colon+1:])
		switch key {
		case "id":
			e.ID = val
		case "category":
			e.Category = val
		case "intent":
			e.Intent = val
		case "tags":
			for _, t := range strings.Split(val, ",") {
				trimmed := strings.TrimSpace(t)
				if trimmed != "" {
					e.Tags = append(e.Tags, trimmed)
				}
			}
		}
	}
}
