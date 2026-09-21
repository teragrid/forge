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

import "testing"

// Values below are taken from a real project's scan output (56 generic-bearer
// findings, all fixtures/docs) so the rule is pinned to real data, not to
// examples invented to fit it.
func TestIsPlaceholderCredential_RealWorldPlaceholders(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, rel, value string
	}{
		// explicit marker word, anywhere in the tree
		{"mock in test", "tests/unit/a.test.ts", "mock-refresh-token"},
		{"mock in setup file", "jest.setup.js", "mock-refresh-token"},
		{"test marker in prod code", "src/lib/oauth/google.ts", "promotiai-test-connection-probe-invalid-token"},
		{"your in docs", "docs/QUICK_APPLY_GUIDE.md", "sbp_your_token_here"},
		{"YOUR upper case in script", "scripts/restart.ps1", "YOUR_ACCESS_TOKEN"},
		{"placeholder word", "src/x.ts", "whsec_placeholder_value"},
		{"invalid marker", "src/x.ts", "invalid_refresh_token"},
		{"do-not-use", "src/webhook.test.ts", "test-secret-do-not-use-in-prod"},
		// no marker, but plain words in test code
		{"phrase in tests dir", "tests/integration/oauth.test.js", "duplicate_secret"},
		{"trailing digits are plain", "tests/unit/oauth.test.ts", "invalid_token_12345"},
		{"digit suffix on a word", "tests/unit/tw.test.ts", "tw-oauth1-access-token"},
		{"new_access_token", "src/test/refresh.test.ts", "new_access_token"},
		{"file-name pattern only", "src/lib/refresh.spec.ts", "brand_new_token_abc123"},
		{"__tests__ dir", "src/__tests__/a.ts", "my_super_secret_value_here"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if !isPlaceholderCredential(c.rel, c.value) {
				t.Fatalf("isPlaceholderCredential(%q, %q) = false; want true", c.rel, c.value)
			}
		})
	}
}

// The important half: anything that looks like a real credential must keep
// being reported, in production code AND in test files.
func TestIsPlaceholderCredential_RealSecretsStillFlagged(t *testing.T) {
	t.Parallel()
	// Secret-shaped values are assembled from fragments so no source line contains a
	// contiguous provider key: GitHub push protection (rightly) cannot tell a fixture
	// from a leak, and blocked the first push of this file.
	secrets := map[string]string{
		"stripe live key":        "sk_" + "live_51HabcDEFghiJKLmnoPQRstu",
		"stripe test key":        "sk_" + "test_51HabcDEFghiJKLmnoPQRstu", // test-MODE keys are still secrets
		"random base62":          "q8Zr3KfL0wXv9TbNc2YpHs7D",
		"hex":                    "3f2a9c1e7b4d40a8b6c5d2e1f0a9b8c7",
		"uuid":                   "3f2a9c1e-7b4d-40a8-b6c5-d2e1f0a9b8c7",
		"jwt header":             "eyJhbGciOiJIUzI1NiIsInR5cCI",
		"real whsec":             "whsec_" + "Xk3Lm9Qw2Ert7Yu1Io5PaSd4Fg",
		"mixed case blend":       "qWeRtY_asDfGh_zXcVbN",
		"letter-digit blend":     "live-tok-a1b2c3d4e5f6g7h8",
		"marker but opaque part": "test_Xk3Lm9Qw2Ert7Yu1Io5PaSd4Fg",
		"single long lower word": "abcdefghijklmnopqrstuvwxyz",
		"camelCase single token": "xClientSecret123abc456def789ghi",
	}
	for name, v := range secrets {
		name, v := name, v
		for _, rel := range []string{"src/lib/auth.ts", "tests/unit/auth.test.ts"} {
			rel := rel
			t.Run(name+" @ "+rel, func(t *testing.T) {
				t.Parallel()
				if isPlaceholderCredential(rel, v) {
					t.Fatalf("isPlaceholderCredential(%q, %q) = true; a real-looking secret must still be reported", rel, v)
				}
			})
		}
	}
}

// A phrase-shaped value with no marker is only excused inside test code. The same
// literal in production code is still a plausible hard-coded password.
func TestIsPlaceholderCredential_PhraseInProductionStillFlagged(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"correct-horse-battery-staple", "my-secret-password-2024", "promotiai-social-inbox-webhook"} {
		if isPlaceholderCredential("src/lib/auth.ts", v) {
			t.Errorf("%q in production code must still be reported", v)
		}
		if !isPlaceholderCredential("tests/unit/auth.test.ts", v) {
			t.Errorf("%q in a test file should be treated as a fixture", v)
		}
	}
}

func TestIsPlaceholderCredential_Boundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		value string
		want  bool
	}{
		{"", false},
		{"testtesttesttesttest", false}, // marker, but one segment: not phrase-shaped
		{"test_", false},                // one segment after splitting
		{"_test_mock_", true},           // leading/trailing separators are ignored
		{"test__mock--fake", true},      // repeated separators collapse
		{"test_1234567", false},         // 7 digits is not a short number
		{"test_123456", true},           // 6 digits is
		{"Test_Mock_Token", true},       // Capitalised words
		{"TEST_MOCK_TOKEN", true},       // UPPER words
		{"tEsT_mock_token", false},      // mixed-case blend
	}
	for _, c := range cases {
		if got := isPlaceholderCredential("src/x.ts", c.value); got != c.want {
			t.Errorf("isPlaceholderCredential(src/x.ts, %q) = %v; want %v", c.value, got, c.want)
		}
	}
}

func TestIsTestPath(t *testing.T) {
	t.Parallel()
	yes := []string{
		"tests/a.js", "test/a.js", "src/__tests__/a.ts", "src/__mocks__/x.ts", "e2e/login.ts",
		"src/lib/a.test.ts", "src/lib/a.spec.js", "pkg/a_test.go", "scripts/test_helpers.py", "src/api.mock.ts",
		"TESTS/A.JS", // case-insensitive
	}
	no := []string{
		"src/lib/a.ts", "src/contest/a.ts", "src/latest/a.ts", "docs/spec/a.md", "src/attestation.ts",
		"src/lib/testify.ts", "a.ts",
	}
	for _, p := range yes {
		if !isTestPath(p) {
			t.Errorf("isTestPath(%q) = false; want true", p)
		}
	}
	for _, p := range no {
		if isTestPath(p) {
			t.Errorf("isTestPath(%q) = true; want false", p)
		}
	}
}
