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

package cmdscan

import (
	"regexp"
	"strings"
)

// The built-in `generic-bearer` rule is a heuristic: any quoted literal of 16+
// [A-Za-z0-9_-] characters assigned to a name containing token/secret/password/
// api-key. On a real project that produces a wall of findings on test fixtures
// ("test_access_token", "whsec_placeholder", "mock-refresh-token") and on
// documentation placeholders ("sbp_your_token_here"), which drowns out real hits
// and makes `forge scan security` permanently red.
//
// isPlaceholderCredential recognises those literals structurally rather than by
// silencing whole directories, so a real secret that happens to sit in a test
// file is still reported:
//
//   - A value is a candidate only if it is a *phrase*: two or more segments split
//     on '-' or '_', and every segment is a plain word (all lower case, all upper
//     case, or Capitalised) optionally followed by up to six digits — or a short
//     run of digits. Random credentials are opaque: they mix upper and lower
//     case, letters and digits inside one segment, or are a single long segment,
//     so they never qualify. That is why live and test-mode Stripe keys, real
//     webhook secrets, hex,
//     UUIDs, base62 and JWT-ish strings are still flagged.
//   - A phrase is then treated as a placeholder only when something says it is
//     not real: it contains an explicit marker word (test, mock, fake, dummy,
//     example, sample, placeholder, invalid, your, …) OR it sits in a test path.
//     A phrase-shaped value in production code with no marker
//     ("correct-horse-battery-staple") is still reported.
//
// Provider-specific rules (AWS keys, sk- keys, GitHub tokens, private-key
// blocks) are not affected: they are checked independently of this heuristic.

// placeholderMarkers are segment words that state, in the value itself, that it
// is not a real credential.
var placeholderMarkers = map[string]struct{}{
	"test": {}, "mock": {}, "fake": {}, "dummy": {}, "example": {}, "sample": {},
	"placeholder": {}, "changeme": {}, "invalid": {}, "your": {}, "redacted": {},
	"xxx": {}, "demo": {}, "fixture": {}, "stub": {},
}

// plainSegment matches one word of a phrase: lower, UPPER or Capitalised
// letters with at most six trailing digits, or a bare run of up to six digits.
// Mixed-case blends such as "qWeRtY" and letter/digit blends such as "a1b2c3"
// do not match, which is what keeps opaque credentials out.
var plainSegment = regexp.MustCompile(`^(?:[a-z]+|[A-Z]+|[A-Z][a-z]+)[0-9]{0,6}$|^[0-9]{1,6}$`)

// testPathDirs are directory names that mark a path as test code.
var testPathDirs = map[string]struct{}{
	"test": {}, "tests": {}, "__tests__": {}, "__mocks__": {}, "mocks": {},
	"e2e": {}, "__fixtures__": {},
}

// isTestPath reports whether rel (slash-separated, relative to the scan root)
// is test code, by directory name or by conventional file-name pattern.
func isTestPath(rel string) bool {
	parts := strings.Split(strings.ToLower(rel), "/")
	for _, dir := range parts[:len(parts)-1] {
		if _, ok := testPathDirs[dir]; ok {
			return true
		}
	}
	base := parts[len(parts)-1]
	return strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") ||
		strings.Contains(base, "_test.") || strings.HasPrefix(base, "test_") ||
		strings.Contains(base, ".mock.")
}

// isPlaceholderCredential reports whether a quoted literal matched by the
// generic-bearer heuristic is recognisably not a real credential. See the
// package comment above for the exact rule and its limits.
func isPlaceholderCredential(rel, value string) bool {
	segments := strings.FieldsFunc(value, func(r rune) bool { return r == '-' || r == '_' })
	if len(segments) < 2 {
		return false
	}
	marked := false
	for _, seg := range segments {
		if !plainSegment.MatchString(seg) {
			return false
		}
		word := strings.ToLower(strings.TrimRight(seg, "0123456789"))
		if _, ok := placeholderMarkers[word]; ok {
			marked = true
		}
	}
	return marked || isTestPath(rel)
}
