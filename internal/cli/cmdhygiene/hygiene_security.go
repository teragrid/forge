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

// Package cmdhygiene — G-062/G-066/G-067/G-068/G-069 hygiene security extensions.
//
// G-062: forge-owner ownership tag stamping and reading.
// G-066: Mandatory hygiene block — validates that .gitignore contains all required forge entries.
// G-067: Negation discipline — validates that .example/.template paths are negated.
// G-068: .gitleaks.toml framework-managed generation.
// G-069: Allowlist expiry gate — reports stale allowlist entries past their review date.
package cmdhygiene

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ── G-062: forge-owner ownership tags ────────────────────────────────────────

const forgeOwnerTag = "forge-owner:"

// OwnerFor reads the forge-owner tag from the first 5 lines of a file.
// Returns ("", false) if no tag is found.
func OwnerFor(root, rel string) (string, bool) {
	path := filepath.Join(root, filepath.FromSlash(rel))
	f, err := os.Open(path) // #nosec G304
	if err != nil {
		return "", false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() && lineNo < 5 {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		// Supported comment styles: //, #, --
		for _, prefix := range []string{"// ", "# ", "-- "} {
			if strings.HasPrefix(line, prefix) {
				rest := strings.TrimPrefix(line, prefix)
				if strings.HasPrefix(rest, forgeOwnerTag) {
					owner := strings.TrimSpace(strings.TrimPrefix(rest, forgeOwnerTag))
					return owner, true
				}
			}
		}
	}
	return "", false
}

// StampOwnership prepends a forge-owner header comment to a file, choosing
// the comment style based on extension. Does nothing if the file already has a
// forge-owner tag.
func StampOwnership(root, rel, verb string) error {
	owner, alreadySet := OwnerFor(root, rel)
	if alreadySet && owner == verb {
		return nil // already correct
	}
	if alreadySet {
		return nil // don't overwrite user-set owner
	}

	path := filepath.Join(root, filepath.FromSlash(rel))
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return fmt.Errorf("stamp ownership read %s: %w", rel, err)
	}

	commentStyle := commentStyleFor(rel)
	header := fmt.Sprintf("%s %s %s\n", commentStyle, forgeOwnerTag, verb)
	newData := append([]byte(header), data...)
	return os.WriteFile(path, newData, 0o644)
}

// commentStyleFor returns the appropriate single-line comment prefix for a file.
func commentStyleFor(rel string) string {
	ext := strings.ToLower(filepath.Ext(rel))
	switch ext {
	case ".go", ".js", ".ts", ".java", ".kt", ".swift", ".c", ".cpp", ".h":
		return "//"
	case ".sql":
		return "--"
	default:
		return "#"
	}
}

// ── G-066: Mandatory hygiene block ───────────────────────────────────────────

// mandatoryGitignoreEntries is the minimum set of entries forge requires in
// .gitignore per spec §4.
var mandatoryGitignoreEntries = []string{
	".env",
	".env.local",
	".env.*.local",
	".forge/cache/",
	".forge/llm-scratch/",
	".forge/trash/",
	".forge/session/",
	".forge/learned/",
	".forge/scan-history/",
	".forge/eval-runs/",
}

// ValidateMandatoryBlock checks that .gitignore in root contains all mandatory
// forge entries. Returns the list of missing entries.
func ValidateMandatoryBlock(root string) ([]string, error) {
	path := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		if os.IsNotExist(err) {
			return mandatoryGitignoreEntries, nil // all missing
		}
		return nil, fmt.Errorf("read .gitignore: %w", err)
	}

	present := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		present[strings.TrimSpace(scanner.Text())] = true
	}

	var missing []string
	for _, entry := range mandatoryGitignoreEntries {
		if !present[entry] {
			missing = append(missing, entry)
		}
	}
	return missing, nil
}

// ── G-067: Negation discipline ────────────────────────────────────────────────

// ValidateNegationDiscipline checks that .example and .template files in the
// project are explicitly re-included via negation in .gitignore. Returns the
// paths that are not covered by a negation rule.
func ValidateNegationDiscipline(root string) ([]string, error) {
	path := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read .gitignore: %w", err)
	}

	negations := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "!") {
			negations[strings.TrimPrefix(line, "!")] = true
		}
	}

	// Check for any *.example or *.template file not covered by a negation.
	var missing []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".example") || strings.HasSuffix(name, ".template") {
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			// Check if this path or its basename is explicitly negated.
			if !negations[rel] && !negations[name] && !negations["*.example"] && !negations["*.template"] {
				missing = append(missing, rel)
			}
		}
		return nil
	})
	return missing, nil
}

// ── G-068: .gitleaks.toml framework-managed ───────────────────────────────────

// gitleaksTemplate is the forge-managed .gitleaks.toml template with
// Forge-specific rules per spec §4.
const gitleaksTemplate = `# .gitleaks.toml — forge-managed (do not remove forge-managed comment block)
# forge-managed: true
# To add custom rules, append them after the forge rules section.

title = "ForgeLeaks"

[[rules]]
id = "forge-supabase-service-role"
description = "Supabase service-role JWT detected"
regex = '''eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+'''
tags = ["supabase", "jwt"]

[[rules]]
id = "forge-openai-key"
description = "OpenAI API key"
regex = '''sk-[A-Za-z0-9]{32,48}'''
tags = ["openai", "ai"]

[[rules]]
id = "forge-anthropic-key"
description = "Anthropic API key"
regex = '''sk-ant-[A-Za-z0-9\-]{32,64}'''
tags = ["anthropic", "ai"]

[[rules]]
id = "forge-google-ai-key"
description = "Google AI / Gemini key"
regex = '''AIza[A-Za-z0-9\-_]{35}'''
tags = ["google", "ai"]

[[rules]]
id = "forge-stripe-live-key"
description = "Stripe live secret key"
regex = '''sk_live_[A-Za-z0-9]{24,64}'''
tags = ["stripe", "payment"]

[[rules]]
id = "forge-twilio-key"
description = "Twilio API key"
regex = '''SK[A-Za-z0-9]{32}'''
tags = ["twilio", "communication"]

[[rules]]
id = "forge-sendgrid-key"
description = "SendGrid API key"
regex = '''SG\.[A-Za-z0-9\-_]{22}\.[A-Za-z0-9\-_]{43}'''
tags = ["sendgrid", "email"]

[allowlist]
# Entries in this list must include a review-by date comment:
#   "path/to/file.txt" # review-by: 2026-12-31
regexes = []
paths = []
commits = []
`

// EnsureGitleaksConfig writes a default .gitleaks.toml to root if it does not
// already exist. An existing file is never overwritten. G-068.
func EnsureGitleaksConfig(root string) error {
	path := filepath.Join(root, ".gitleaks.toml")
	if _, err := os.Stat(path); err == nil {
		return nil // already present
	}
	return os.WriteFile(path, []byte(gitleaksTemplate), 0o644)
}

// ── G-069: Allowlist expiry gate ─────────────────────────────────────────────

// AllowlistEntry represents one expired or expiring allowlist entry.
type AllowlistEntry struct {
	Path       string
	ReviewBy   time.Time
	DaysStale  int
}

// CheckAllowlistExpiry reads .gitleaks.toml and returns any allowlist entries
// whose review-by date has passed. Lines must include a trailing comment in the
// form: # review-by: YYYY-MM-DD. G-069.
func CheckAllowlistExpiry(root string) ([]AllowlistEntry, error) {
	path := filepath.Join(root, ".gitleaks.toml")
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read .gitleaks.toml: %w", err)
	}

	now := time.Now().UTC()
	var expired []AllowlistEntry

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.Index(line, "# review-by:")
		if idx < 0 {
			continue
		}
		rawDate := strings.TrimSpace(line[idx+len("# review-by:"):])
		// Trim any trailing quote or whitespace.
		rawDate = strings.Trim(rawDate, `"' `)
		t, parseErr := time.Parse("2006-01-02", rawDate)
		if parseErr != nil {
			continue
		}
		if t.Before(now) {
			entryPath := strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
			entryPath = strings.Trim(entryPath, `"' `)
			expired = append(expired, AllowlistEntry{
				Path:      entryPath,
				ReviewBy:  t,
				DaysStale: int(now.Sub(t).Hours() / 24),
			})
		}
	}
	return expired, nil
}
