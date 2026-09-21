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
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func future() string { return time.Now().AddDate(0, 1, 0).Format("2006-01-02") }
func past() string   { return time.Now().AddDate(0, 0, -3).Format("2006-01-02") }

func findingsFixture() *ScanResult {
	r := &ScanResult{Findings: []Finding{
		{File: "src/lib/webhookConfig.ts", Line: 17, Rule: "generic-bearer"},
		{File: "tests/integration/x.test.js", Line: 4, Rule: "generic-bearer"},
		{File: "src/lib/webhookConfig.ts", Line: 30, Rule: "aws-access-key"},
	}}
	finalizeStatus(r)
	return r
}

func TestApplyWaivers_NoDirIsNoop(t *testing.T) {
	t.Parallel()
	res := findingsFixture()
	if err := ApplyWaivers(t.TempDir(), res); err != nil {
		t.Fatalf("ApplyWaivers: %v", err)
	}
	if len(res.Findings) != 3 || res.Waived != 0 || res.Note != "" {
		t.Fatalf("no waivers dir must change nothing; got %+v", res)
	}
}

func TestApplyWaivers_FileScopedWaiverRemovesOnlyThatFileAndRule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, ".forge/waivers/w.yml", `
- id: W-001
  rule_id: generic-bearer
  file_path: src/lib/webhookConfig.ts
  rationale: "Public Meta webhook verify token; documented as not a secret."
  approved_by: trung
  expires_at: "`+future()+`"
`)
	res := findingsFixture()
	if err := ApplyWaivers(root, res); err != nil {
		t.Fatalf("ApplyWaivers: %v", err)
	}
	if res.Waived != 1 || len(res.Findings) != 2 || res.Count != 2 {
		t.Fatalf("want 1 waived and 2 left; got waived=%d findings=%+v", res.Waived, res.Findings)
	}
	for _, f := range res.Findings {
		if f.File == "src/lib/webhookConfig.ts" && f.Rule == "generic-bearer" {
			t.Fatalf("waived finding still present: %+v", f)
		}
	}
	// a DIFFERENT rule in the same file must not be waived
	stillThere := false
	for _, f := range res.Findings {
		if f.Rule == "aws-access-key" {
			stillThere = true
		}
	}
	if !stillThere {
		t.Fatal("waiver for generic-bearer must not suppress aws-access-key in the same file")
	}
	if !strings.Contains(res.Note, "1 finding(s) waived") {
		t.Fatalf("note should say how many were waived; got %q", res.Note)
	}
}

func TestApplyWaivers_RuleWideWaiver(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, ".forge/waivers/w.yml", `
id: W-002
rule_id: generic-bearer
rationale: "accepted repo-wide while migrating secrets to a vault"
approved_by: trung
expires_at: "`+future()+`"
`)
	res := findingsFixture()
	if err := ApplyWaivers(root, res); err != nil {
		t.Fatalf("ApplyWaivers: %v", err)
	}
	if res.Waived != 2 || len(res.Findings) != 1 || res.Findings[0].Rule != "aws-access-key" {
		t.Fatalf("rule-wide waiver should remove both generic-bearer findings only; got waived=%d %+v", res.Waived, res.Findings)
	}
	if res.Status != "suspicious" {
		t.Fatalf("status should be recomputed for 1 finding; got %q", res.Status)
	}
}

func TestApplyWaivers_ExpiredWaiverIsNotHonoured(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, ".forge/waivers/w.yml", `
- id: W-OLD
  rule_id: generic-bearer
  file_path: src/lib/webhookConfig.ts
  rationale: "temporary"
  approved_by: trung
  expires_at: "`+past()+`"
`)
	res := findingsFixture()
	if err := ApplyWaivers(root, res); err != nil {
		t.Fatalf("ApplyWaivers: %v", err)
	}
	if res.Waived != 0 || len(res.Findings) != 3 {
		t.Fatalf("expired waiver must not suppress anything; got waived=%d findings=%d", res.Waived, len(res.Findings))
	}
	if !strings.Contains(res.Note, "expired waiver") || !strings.Contains(res.Note, "W-OLD") {
		t.Fatalf("expiry must be visible in the note; got %q", res.Note)
	}
}

// Renewing a waiver by adding a new entry (without deleting the lapsed one) must work.
func TestApplyWaivers_ValidWaiverBeatsExpiredDuplicate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, ".forge/waivers/a_old.yml", `
- id: W-OLD
  rule_id: generic-bearer
  file_path: src/lib/webhookConfig.ts
  rationale: "first approval"
  approved_by: trung
  expires_at: "`+past()+`"
`)
	writeFile(t, root, ".forge/waivers/b_renewed.yml", `
- id: W-NEW
  rule_id: generic-bearer
  file_path: src/lib/webhookConfig.ts
  rationale: "re-approved after review"
  approved_by: trung
  expires_at: "`+future()+`"
`)
	res := findingsFixture()
	if err := ApplyWaivers(root, res); err != nil {
		t.Fatalf("ApplyWaivers: %v", err)
	}
	if res.Waived != 1 || strings.Contains(res.Note, "expired") {
		t.Fatalf("renewed waiver should apply cleanly; got waived=%d note=%q", res.Waived, res.Note)
	}
}

// Fail closed: a waiver without a reason, approver or expiry is an error, not a
// silent exemption.
func TestApplyWaivers_IncompleteWaiverFailsClosed(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"no rationale":    "id: W-1\nrule_id: generic-bearer\napproved_by: t\nexpires_at: \"" + future() + "\"\n",
		"no approved_by":  "id: W-2\nrule_id: generic-bearer\nrationale: r\nexpires_at: \"" + future() + "\"\n",
		"no expires_at":   "id: W-3\nrule_id: generic-bearer\nrationale: r\napproved_by: t\n",
		"blank rationale": "id: W-4\nrule_id: generic-bearer\nrationale: \"   \"\napproved_by: t\nexpires_at: \"" + future() + "\"\n",
	}
	for name, body := range cases {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeFile(t, root, ".forge/waivers/w.yml", body)
			res := findingsFixture()
			err := ApplyWaivers(root, res)
			if err == nil {
				t.Fatal("expected an error for an incomplete waiver")
			}
			if len(res.Findings) != 3 || res.Waived != 0 {
				t.Fatalf("an invalid waiver must suppress nothing; got waived=%d findings=%d", res.Waived, len(res.Findings))
			}
		})
	}
}

func TestApplyWaivers_MalformedYAMLIsAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, ".forge/waivers/w.yml", "id: [unterminated\n")
	if err := ApplyWaivers(root, findingsFixture()); err == nil {
		t.Fatal("expected an error for malformed waiver YAML")
	}
}

// CLI level: what CI actually runs. Same tree, exit code flips only because of the waiver.
func TestScanCommand_WaiverTurnsGateGreen_AndJSONReportsWaived(t *testing.T) {
	t.Parallel()
	if hasGitleaks() {
		t.Skip("gitleaks installed; built-in patterns not exercised")
	}
	root := t.TempDir()
	writeFile(t, root, "src/lib/webhookConfig.ts",
		"export const META_WEBHOOK_VERIFY_TOKEN = 'promotiai-social-inbox-webhook';\n")

	run := func() (string, error) {
		cmd := New()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"secrets", "--root", root, "--json"})
		err := cmd.Execute()
		return out.String(), err
	}

	if _, err := run(); err == nil {
		t.Fatal("precondition: the un-waived finding must fail the gate")
	}

	writeFile(t, root, ".forge/waivers/webhook.yml", `
id: W-WEBHOOK
rule_id: generic-bearer
file_path: src/lib/webhookConfig.ts
rationale: "Public Meta webhook verify token; the file documents it is not a secret."
approved_by: trung
expires_at: "`+future()+`"
`)
	out, err := run()
	if err != nil {
		t.Fatalf("waived finding must not fail the gate: %v\n%s", err, out)
	}
	var got ScanResult
	if jerr := json.Unmarshal([]byte(out), &got); jerr != nil {
		t.Fatalf("json: %v\n%s", jerr, out)
	}
	if got.Waived != 1 || got.Count != 0 || got.Status != "clean" {
		t.Fatalf("want waived=1 count=0 clean; got %+v", got)
	}
}

func TestScanCommand_InvalidWaiverFailsTheScan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "src/a.go", "package a\n")
	writeFile(t, root, ".forge/waivers/w.yml", "id: W-X\nrule_id: generic-bearer\n")
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"secrets", "--root", root})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("a scan with an incomplete waiver must fail closed; output: %s", out.String())
	}
}
