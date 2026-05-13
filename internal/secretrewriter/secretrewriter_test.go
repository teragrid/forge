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

package secretrewriter_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/secretrewriter"
)

// ── Happy path ────────────────────────────────────────────────────────────────

func TestRewrite_OpenAIKey(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	input := "Use key sk-abcdefghijklmnopqrstuvwxyz1234567890 to call the API."
	res := r.Rewrite(input)
	if strings.Contains(res.Text, "sk-abcdefghijklmnopqrstuvwxyz1234567890") {
		t.Errorf("OpenAI key still present in output: %q", res.Text)
	}
	if res.Replacements == 0 {
		t.Error("expected at least one replacement")
	}
}

func TestRewrite_AnthropicKey(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	input := "Authorization: Bearer sk-ant-api03-abcdefghijklmnopqrstuvwxyz12345678"
	res := r.Rewrite(input)
	if strings.Contains(res.Text, "sk-ant-api03") {
		t.Errorf("Anthropic key still present: %q", res.Text)
	}
}

func TestRewrite_JWT(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	// A syntactically valid JWT (header.payload.signature — all base64url)
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyMTIzIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	input := "token=" + jwt
	res := r.Rewrite(input)
	if strings.Contains(res.Text, jwt) {
		t.Errorf("JWT still present: %q", res.Text)
	}
}

func TestRewrite_GitHubToken(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	input := "GITHUB_TOKEN=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"
	res := r.Rewrite(input)
	if strings.Contains(res.Text, "ghp_") {
		t.Errorf("GitHub token still present: %q", res.Text)
	}
}

func TestRewrite_AWSKey(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	input := "aws_access_key_id = AKIAIOSFODNN7EXAMPLE"
	res := r.Rewrite(input)
	if strings.Contains(res.Text, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("AWS key still present: %q", res.Text)
	}
}

// ── Placeholder length independence ──────────────────────────────────────────

func TestRewrite_PlaceholderLengthIsConstant(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	short := "sk-shortkeytoolongtobe12345678901"
	long := "sk-" + strings.Repeat("a", 60)
	res1 := r.Rewrite(short)
	res2 := r.Rewrite(long)
	// After scrubbing the key part is the same placeholder regardless of key length.
	if !strings.Contains(res1.Text, "[REDACTED:") {
		t.Errorf("short key not replaced: %q", res1.Text)
	}
	if !strings.Contains(res2.Text, "[REDACTED:") {
		t.Errorf("long key not replaced: %q", res2.Text)
	}
}

// ── Idempotency ───────────────────────────────────────────────────────────────

func TestRewrite_Idempotent(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	input := "key=sk-abcdefghijklmnopqrstuvwxyz1234567890"
	once := r.Rewrite(input)
	twice := r.Rewrite(once.Text)
	if once.Text != twice.Text {
		t.Errorf("rewrite is not idempotent:\nonce=%q\ntwice=%q", once.Text, twice.Text)
	}
}

// ── Custom pattern ────────────────────────────────────────────────────────────

func TestRewrite_CustomPattern(t *testing.T) {
	t.Parallel()
	custom := regexp.MustCompile(`\bFORGE-SECRET-[A-Z0-9]{16}\b`)
	r := secretrewriter.New(custom)
	input := "config FORGE-SECRET-ABCDEFGH12345678 applied"
	res := r.Rewrite(input)
	if strings.Contains(res.Text, "FORGE-SECRET-ABCDEFGH12345678") {
		t.Errorf("custom secret still present: %q", res.Text)
	}
}

// ── False-positive guard ──────────────────────────────────────────────────────

// A normal English sentence that superficially resembles a key should NOT be redacted.
func TestRewrite_ShortTokenNotRedacted(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	// "sk-short" is only 8 chars after "sk-", below the 20-char threshold.
	input := "Use tag sk-short for internal labelling."
	res := r.Rewrite(input)
	if res.Replacements > 0 {
		t.Errorf("short sk- token should NOT be redacted, got: %q", res.Text)
	}
}

func TestRewrite_PlainText_NoRedaction(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	plain := "Hello, world! The answer is 42."
	res := r.Rewrite(plain)
	if res.Text != plain {
		t.Errorf("plain text was modified: %q", res.Text)
	}
	if res.Replacements != 0 {
		t.Errorf("expected 0 replacements, got %d", res.Replacements)
	}
}

// ── Boundary: empty input ─────────────────────────────────────────────────────

func TestRewrite_Empty(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	res := r.Rewrite("")
	if res.Text != "" {
		t.Errorf("empty input should produce empty output, got %q", res.Text)
	}
}

// ── Multiple secrets in one string ────────────────────────────────────────────

func TestRewrite_MultipleSecrets(t *testing.T) {
	t.Parallel()
	r := secretrewriter.New()
	input := "key1=sk-abcdefghijklmnopqrstuvwxyz123456 key2=sk-zyxwvutsrqponmlkjihgfedcba654321"
	res := r.Rewrite(input)
	if strings.Contains(res.Text, "sk-abc") || strings.Contains(res.Text, "sk-zyx") {
		t.Errorf("not all secrets were redacted: %q", res.Text)
	}
	if res.Replacements < 2 {
		t.Errorf("expected >= 2 replacements, got %d", res.Replacements)
	}
}

// ── DEV-M0-26 100-run corpus regression ──────────────────────────────────────

// TestRewriter_100RunRegression seeds a corpus of known secrets, runs Rewrite()
// 100 times on each, and asserts that every secret is redacted on every run.
// This guards against flaky regex engine behaviour or race conditions.
func TestRewriter_100RunRegression(t *testing.T) {
	t.Parallel()

	type corpus struct {
		name    string
		input   string
		markers []string // substrings that must NOT appear in output
		extra   []*regexp.Regexp
	}

	cases := []corpus{
		{
			name:    "openai-key",
			input:   "OPENAI_API_KEY=sk-abcdefghijklmnopqrstuvwxyz1234567890AAAAAA",
			markers: []string{"sk-abcdefghijklmnopqrstuvwxyz"},
		},
		{
			name:    "anthropic-key",
			input:   "ANTHROPIC_API_KEY=sk-ant-api03-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOP1234",
			markers: []string{"sk-ant-api03"},
		},
		{
			name:  "aws-access-key",
			input: "aws_access_key_id = AKIAIOSFODNN7EXAMPLE",
			// Exactly AKIA + 16 uppercase alphanum chars — matches \bAKIA[A-Z0-9]{16}\b
			markers: []string{"AKIAIOSFODNN7EXAMPLE"},
		},
		{
			name:    "github-token",
			input:   "GITHUB_TOKEN=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij",
			markers: []string{"ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
		},
		{
			name:    "jwt-token",
			input:   "token=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0dXNlcjEyMyIsImFkbWluIjp0cnVlfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			markers: []string{"eyJhbGciOiJIUzI1NiJ9"},
		},
		{
			// Stripe live key uses sk_live_ (underscore) — not matched by the sk- pattern;
			// requires a custom pattern. Tests that custom patterns also hold for 100 runs.
			name:    "stripe-live-key-custom",
			input:   "STRIPE_SECRET_KEY=sk_live_abcdefghijklmnopqrstuvwxyz1234567890AB",
			markers: []string{"sk_live_abcdefghijklmnopqrstuvwxyz"},
			extra:   []*regexp.Regexp{regexp.MustCompile(`\bsk_live_[A-Za-z0-9_]{20,}\b`)},
		},
		{
			// Slack bot token uses xoxb- prefix — requires a custom pattern.
			name:    "slack-bot-token-custom",
			input:   "SLACK_BOT_TOKEN=xoxb-123456789012-1234567890123-abcdefghijklmnopqrstuvwxyz",
			markers: []string{"xoxb-"},
			extra:   []*regexp.Regexp{regexp.MustCompile(`\bxoxb-[0-9A-Za-z\-]{20,}\b`)},
		},
		{
			name:    "multi-secret-line",
			input:   "key1=sk-abcdefghijklmnopqrstuvwxyz123456 key2=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef",
			markers: []string{"sk-abcdefghijklmnopqrstuvwxyz123456", "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
		},
	}

	const runs = 100

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := secretrewriter.New(tc.extra...)
			for i := range runs {
				res := r.Rewrite(tc.input)
				for _, marker := range tc.markers {
					if strings.Contains(res.Text, marker) {
						t.Errorf("run %d/%d: secret marker %q still present in output:\n  input:  %q\n  output: %q",
							i+1, runs, marker, tc.input, res.Text)
					}
				}
				if res.Replacements == 0 {
					t.Errorf("run %d/%d: expected >=1 replacement, got 0 for %q", i+1, runs, tc.name)
				}
			}
		})
	}
}
