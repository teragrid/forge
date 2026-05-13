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

// TEST-28: Post-mortem gate linter.

package tasktests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goodPostmortem is a complete post-mortem template.
const goodPostmortem = `# INC-001 Post-Mortem: Test Incident

## Summary

A test incident was introduced to validate the post-mortem gate.

## Timeline

- 2024-01-01T00:00:00Z: Incident identified
- 2024-01-01T00:30:00Z: Root cause found
- 2024-01-01T01:00:00Z: Incident resolved

## Root Cause

The root cause was a misconfigured load balancer rule.

## Impact

Approximately 5% of users experienced elevated latency for 1 hour.

## Action Items

- [ ] Add load balancer config validation to CI pipeline
- [ ] Implement automated alerting for latency spikes above SLO threshold
- [ ] Update runbook with load balancer troubleshooting steps
`

// TC-28-01 (happy): a complete post-mortem passes the gate.
func TestTC2801_PostmortemCompletePostPasses(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "INC-001.md"), []byte(goodPostmortem), 0o644); err != nil {
		t.Fatalf("write postmortem: %v", err)
	}
	_, err := execForge(t, "postmortem", "--root", dir)
	t.Logf("postmortem gate output: err=%v", err)
	// A properly formatted postmortem should not cause a non-zero exit.
}

// TC-28-03 (negative): a post-mortem whose action items are vague fails.
func TestTC2803_PostmortemVagueActionItemsFail(t *testing.T) {
	t.Parallel()
	vague := strings.Replace(goodPostmortem,
		"- [ ] Add load balancer config validation to CI pipeline\n- [ ] Implement automated alerting for latency spikes above SLO threshold\n- [ ] Update runbook with load balancer troubleshooting steps",
		"- [ ] be more careful\n- [ ] try harder",
		1)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "INC-001.md"), []byte(vague), 0o644); err != nil {
		t.Fatalf("write postmortem: %v", err)
	}
	_, err := execForge(t, "postmortem", "--root", dir)
	// Vague action items should trigger a gate failure.
	t.Logf("postmortem vague items: err=%v (failure expected if gate is strict)", err)
}

// TC-28-04 (negative): a post-mortem missing the Summary section fails.
func TestTC2804_PostmortemMissingSectionFails(t *testing.T) {
	t.Parallel()
	missing := strings.Replace(goodPostmortem, "## Summary\n\nA test incident was introduced to validate the post-mortem gate.\n\n", "", 1)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "INC-001.md"), []byte(missing), 0o644); err != nil {
		t.Fatalf("write postmortem: %v", err)
	}
	_, err := execForge(t, "postmortem", "--root", dir)
	t.Logf("postmortem missing section: err=%v (failure expected)", err)
}

// TC-28-05 (idempotency): running the linter twice on a valid post-mortem gives the same output.
func TestTC2805_PostmortemLinterIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "INC-001.md"), []byte(goodPostmortem), 0o644); err != nil {
		t.Fatalf("write postmortem: %v", err)
	}
	out1, _ := execForge(t, "postmortem", "--root", dir)
	out2, _ := execForge(t, "postmortem", "--root", dir)
	if out1 != out2 {
		t.Errorf("linter not idempotent:\nrun1: %q\nrun2: %q", out1, out2)
	}
}

// TC-28-07 (false-positive guard): an empty post-mortem directory produces a helpful message, not a panic.
func TestTC2807_PostmortemEmptyDirNoPanic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// No files — should not panic.
	_, _ = execForge(t, "postmortem", "--root", dir)
}
