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
	"strings"
	"testing"
)

func genericBearerFiles(res *ScanResult) []string {
	var out []string
	for _, f := range res.Findings {
		if f.Rule == "generic-bearer" {
			out = append(out, f.File)
		}
	}
	return out
}

// TC-FP-07: fixture and documentation placeholders must NOT trigger generic-bearer.
// On a real project these were 56 findings that kept `forge scan security` red.
func TestRunSecrets_GenericBearerPlaceholders_NoFinding(t *testing.T) {
	t.Parallel()
	if hasGitleaks() {
		t.Skip("gitleaks installed; built-in patterns not exercised")
	}
	root := t.TempDir()
	writeFile(t, root, "tests/unit/oauth.test.ts",
		"const a = { access_token: 'test_access_token' };\n"+
			"process.env.STRIPE_WEBHOOK_SECRET = 'whsec_placeholder';\n"+
			"const b = { password: 'test-password-12345' };\n")
	writeFile(t, root, "jest.setup.js", "const x = { refresh_token: 'mock-refresh-token' };\n")
	writeFile(t, root, "docs/GUIDE.md", "export SUPABASE_ACCESS_TOKEN='sbp_your_token_here'\n")
	writeFile(t, root, "scripts/restart.ps1", `$headers = @{ token = "YOUR_ACCESS_TOKEN" }`+"\n")
	writeFile(t, root, "src/lib/probe.ts", "refresh_token: 'promotiai-test-connection-probe-invalid-token',\n")

	res, err := RunSecrets(root)
	if err != nil {
		t.Fatalf("RunSecrets: %v", err)
	}
	if got := genericBearerFiles(res); len(got) != 0 {
		t.Fatalf("placeholders must not be reported; got findings in: %v (%+v)", got, res.Findings)
	}
}

// TC-FP-08: a real-looking secret is still reported even when it sits in a test
// file, and a phrase-shaped hard-coded value is still reported in production code.
func TestRunSecrets_GenericBearerRealSecretsStillFlagged(t *testing.T) {
	t.Parallel()
	if hasGitleaks() {
		t.Skip("gitleaks installed; built-in patterns not exercised")
	}
	root := t.TempDir()
	writeFile(t, root, "tests/unit/leak.test.ts", `const key = { secret: "q8Zr3KfL0wXv9TbNc2YpHs7D" };`+"\n")
	writeFile(t, root, "tests/unit/stripe.test.ts", `const k = { api_key: "sk_`+"test_51HabcDEFghiJKLmnoPQRstu"+`" };`+"\n")
	writeFile(t, root, "src/lib/auth.ts", `const cfg = { password: "correct-horse-battery-staple" };`+"\n")

	res, err := RunSecrets(root)
	if err != nil {
		t.Fatalf("RunSecrets: %v", err)
	}
	got := strings.Join(genericBearerFiles(res), ",")
	for _, want := range []string{"tests/unit/leak.test.ts", "tests/unit/stripe.test.ts", "src/lib/auth.ts"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected a generic-bearer finding in %s; got: %s", want, got)
		}
	}
}

// TC-FP-09: the placeholder heuristic applies to generic-bearer ONLY. Provider-specific
// rules keep firing inside test files.
func TestRunSecrets_ProviderRulesUnaffectedInTestFiles(t *testing.T) {
	t.Parallel()
	if hasGitleaks() {
		t.Skip("gitleaks installed; built-in patterns not exercised")
	}
	root := t.TempDir()
	writeFile(t, root, "tests/unit/aws.test.ts", "const k = 'AKIAIOSFODNN7EXAMPLE';\n")
	writeFile(t, root, "tests/unit/pk.test.ts", "-----BEGIN RSA PRIVATE KEY-----\n")
	writeFile(t, root, "tests/unit/gh.test.ts", "const t = 'ghp_"+strings.Repeat("a1B2", 8)+"';\n")

	res, err := RunSecrets(root)
	if err != nil {
		t.Fatalf("RunSecrets: %v", err)
	}
	rules := map[string]bool{}
	for _, f := range res.Findings {
		rules[f.Rule] = true
	}
	for _, want := range []string{"aws-access-key", "private-key-block", "github-token"} {
		if !rules[want] {
			t.Errorf("rule %s must still fire in test files; findings: %+v", want, res.Findings)
		}
	}
}

// The finding's Secret field must be the same text as before the change so
// existing baselines, waivers and JSON consumers keep working.
func TestRunSecrets_GenericBearerSecretFieldUnchanged(t *testing.T) {
	t.Parallel()
	if hasGitleaks() {
		t.Skip("gitleaks installed; built-in patterns not exercised")
	}
	root := t.TempDir()
	writeFile(t, root, "src/config.go", `token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9_abcXYZ0123"`+"\n")
	res, err := RunSecrets(root)
	if err != nil {
		t.Fatalf("RunSecrets: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("want exactly 1 finding, got %+v", res.Findings)
	}
	if want := `token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9_abcXYZ0123"`; res.Findings[0].Secret != want {
		t.Fatalf("Secret = %q; want the full matched text %q", res.Findings[0].Secret, want)
	}
}
