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

// TEST-12: Eval scenario: secret-redaction (100-run zero-leak).
// TEST-25: Secret-redaction privacy invariant.

package tasktests

import (
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/secretrewriter"
)

// ── TEST-12: Secret-redaction 100-run zero-leak ───────────────────────────────

// TC-12-01 (happy): seeded secrets never appear verbatim in output across 100 runs.
func TestTC1201_SecretRedactionZeroLeak(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	secrets := []string{
		"sk-abcdefghijklmnopqrstuvwxyz123456",
		"sk-ant-abcdefghijklmnopqrstuvwxyz",
		"ghp_abcdefghijklmnopqrstuvwxyz12345",
		"AKIAIOSFODNN7EXAMPLE",
	}
	for i := 0; i < 100; i++ {
		for _, secret := range secrets {
			input := "sending payload: " + secret + " end"
			result := r.Rewrite(input)
			if strings.Contains(result.Text, secret) {
				t.Errorf("run %d: secret %q leaked in output %q", i, secret, result.Text)
			}
			if result.Replacements == 0 {
				t.Errorf("run %d: secret %q was not replaced (0 replacements)", i, secret)
			}
		}
	}
}

// TC-12-02 (boundary): a secret at the minimum-length threshold is still redacted.
// OpenAI keys: sk- + 20 alphanum chars (minimum 20).
func TestTC1202_SecretMinLengthBoundary(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	// Exactly 20 chars after sk- — minimum for the OpenAI pattern.
	// "abcdefghijklmnopqrst" = 20 chars
	minKey := "sk-abcdefghijklmnopqrst" // sk- + 20 chars
	result := r.Rewrite("key: " + minKey)
	if strings.Contains(result.Text, minKey) {
		t.Errorf("minimum-length key %q was not redacted; output: %q", minKey, result.Text)
	}
}

// TC-12-03 (false-positive guard): a non-secret string is NOT redacted.
func TestTC1203_SecretFalsePositiveGuard(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	nonSecret := "hello world this is a normal string"
	result := r.Rewrite(nonSecret)
	if result.Replacements != 0 {
		t.Errorf("non-secret string got %d replacements; output: %q", result.Replacements, result.Text)
	}
	if result.Text != nonSecret {
		t.Errorf("non-secret string was modified: %q → %q", nonSecret, result.Text)
	}
}

// TC-12-04 (data-accuracy): redacted placeholder uses [REDACTED:<type>] format;
// original value does not appear.
func TestTC1204_SecretRedactedPlaceholderFormat(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	secret := "sk-abcdefghijklmnopqrstuvwxyz123456"
	result := r.Rewrite(secret)
	if strings.Contains(result.Text, secret) {
		t.Errorf("secret leaked in output: %q", result.Text)
	}
	if !strings.Contains(result.Text, "[REDACTED:") {
		t.Errorf("output does not contain [REDACTED: prefix: %q", result.Text)
	}
}

// TC-12-05 (regression): base64-encoded form of the secret is caught by entropy fallback.
// Note: base64-encoded secrets that happen to look like high-entropy env vars
// will be caught by the env-secret pattern. We test the JWT pattern here as a proxy.
func TestTC1205_SecretBase64Variant(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	// A JWT token — three base64url segments — should be redacted.
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyQGV4YW1wbGUuY29tIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	result := r.Rewrite("token=" + jwt)
	if strings.Contains(result.Text, jwt) {
		t.Errorf("JWT was not redacted; output: %q", result.Text)
	}
}

// ── TEST-25: Secret-redaction privacy invariant ───────────────────────────────

// TC-25-01 (happy): output contains [REDACTED:...] not the raw secret.
func TestTC2501_PrivacyInvariantRedactedOnly(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	secret := "sk-abcdefghijklmnopqrstuvwxyz123456"
	payload := "path/to/file:42: found key=" + secret + " in config"
	result := r.Rewrite(payload)
	if strings.Contains(result.Text, secret) {
		t.Errorf("raw secret appears in output: %q", result.Text)
	}
}

// TC-25-02 (negative): even with an explicit match, the raw value is not in output.
func TestTC2502_PrivacyRawMatchNotLogged(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	secret := "ghp_abcdefghijklmnopqrstuvwxyz12345"
	result := r.Rewrite("debug: raw match is " + secret)
	if strings.Contains(result.Text, secret) {
		t.Errorf("raw secret leaked in debug output: %q", result.Text)
	}
}

// TC-25-04 (regression): idempotent re-run does not alter already-redacted output.
func TestTC2504_PrivacyIdempotentRedaction(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	secret := "AKIAIOSFODNN7EXAMPLE"
	first := r.Rewrite(secret)
	// Second pass on already-redacted text.
	second := r.Rewrite(first.Text)
	if second.Replacements != 0 {
		t.Errorf("second pass made %d replacements on already-redacted text", second.Replacements)
	}
	if first.Text != second.Text {
		t.Errorf("second pass altered text:\nbefore: %q\nafter:  %q", first.Text, second.Text)
	}
}

// TC-25-05 (false-positive guard): a non-secret string that resembles a key shape
// (but too short) is NOT redacted.
func TestTC2505_PrivacyShortStringNotRedacted(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	// sk- prefix but only 5 chars — below the 20-char minimum for the OpenAI pattern.
	notASecret := "sk-abc"
	result := r.Rewrite(notASecret)
	if result.Replacements != 0 {
		t.Errorf("short non-secret %q was unexpectedly redacted: %q", notASecret, result.Text)
	}
}
