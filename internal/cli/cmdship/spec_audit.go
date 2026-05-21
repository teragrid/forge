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

// TG-38: spec-vs-code audit engine.
//
// auditSpecVsCode checks whether the spec artefacts under .forge/specs/<slug>/
// are consistent with the implementation:
//
//  1. All tasks in tasks.md are checked off (blocking).
//  2. Every authz role declared in spec.yml.authz_model appears in a *.rls.test.ts (blocking).
//  3. Every event in spec.yml.events_emitted is asserted in <slug>.test.ts (warning).
//
// This is wired into checkVerify (checkpoint 5) and also emits gap.detected
// NDJSON events when opts.EventWriter is set.
package cmdship

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// AuditGap is a single detected gap between spec artefacts and the implementation.
type AuditGap struct {
	// Type classifies the gap: "incomplete-tasks" | "authz-role-untested" | "missing-event-test".
	Type string
	// Severity is "blocking" (prevents ship) or "warning" (informational).
	Severity string
	// Description is a human-readable summary of the gap.
	Description string
	// File is the most relevant file path for context (may be empty).
	File string
	// Hint is an actionable remediation step.
	Hint string
}

// SpecAuditResult is returned by auditSpecVsCode.
type SpecAuditResult struct {
	// Slug is the spec directory name that was audited.
	Slug string
	// SpecDir is the absolute path to the spec directory.
	SpecDir string
	// SpecFound is true when spec.md exists in SpecDir.
	SpecFound bool
	// Gaps contains all detected gaps, ordered blocking-first.
	Gaps []AuditGap
}

// HasBlockingGaps returns true when at least one gap has Severity "blocking".
func (r *SpecAuditResult) HasBlockingGaps() bool {
	for _, g := range r.Gaps {
		if g.Severity == "blocking" {
			return true
		}
	}
	return false
}

// auditSpecVsCode checks the spec artefacts for the given description (or all
// specs when description is empty) and returns the combined audit result.
// It never panics and returns an empty result when no .forge/specs/ directory
// exists (i.e., the project has not run forge ship spec yet).
func auditSpecVsCode(root, description string) SpecAuditResult {
	specsDir := filepath.Join(root, ".forge", "specs")

	var slugs []string
	if description != "" {
		slugs = []string{slugify(description)}
	} else {
		entries, err := os.ReadDir(specsDir)
		if err != nil {
			return SpecAuditResult{} // no .forge/specs/ — nothing to audit
		}
		for _, e := range entries {
			if e.IsDir() {
				slugs = append(slugs, e.Name())
			}
		}
	}

	if len(slugs) == 0 {
		return SpecAuditResult{}
	}

	// Audit each slug and merge into a combined result.
	var combined SpecAuditResult
	for _, slug := range slugs {
		r := auditSlug(root, specsDir, slug)
		if r.SpecFound {
			combined.SpecFound = true
		}
		combined.Gaps = append(combined.Gaps, r.Gaps...)
		if combined.Slug == "" {
			combined.Slug = r.Slug
			combined.SpecDir = r.SpecDir
		}
	}
	return combined
}

// auditSlug audits a single spec slug directory.
func auditSlug(root, specsDir, slug string) SpecAuditResult {
	res := SpecAuditResult{
		Slug:    slug,
		SpecDir: filepath.Join(specsDir, slug),
	}
	specMDPath := filepath.Join(specsDir, slug, "spec.md")
	if _, err := os.Stat(specMDPath); err != nil {
		return res // spec.md absent — skip
	}
	res.SpecFound = true

	// 1. Incomplete tasks (blocking).
	res.Gaps = append(res.Gaps, checkIncompleteTasks(filepath.Join(specsDir, slug, "tasks.md"))...)

	// 2. Authz roles not covered by RLS tests (blocking).
	res.Gaps = append(res.Gaps, checkAuthzCoverage(root, filepath.Join(specsDir, slug, "spec.yml"))...)

	// 3. Events emitted not asserted in test file (warning).
	res.Gaps = append(res.Gaps, checkEventCoverage(root, slug, filepath.Join(specsDir, slug, "spec.yml"))...)

	return res
}

// checkIncompleteTasks counts unchecked "- [ ]" / "* [ ]" lines in tasks.md.
func checkIncompleteTasks(tasksPath string) []AuditGap {
	data, err := os.ReadFile(tasksPath)
	if err != nil {
		return nil // tasks.md absent — checkBreakdown handles that separately
	}
	var count int
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "- [ ]") || strings.HasPrefix(line, "* [ ]") {
			count++
		}
	}
	if count == 0 {
		return nil
	}
	return []AuditGap{{
		Type:        "incomplete-tasks",
		Severity:    "blocking",
		Description: fmt.Sprintf("%d task(s) not checked off in tasks.md", count),
		File:        tasksPath,
		Hint:        "complete all tasks (mark [x]) before running forge ship ship",
	}}
}

// checkAuthzCoverage verifies every role in spec.yml.authz_model has coverage
// in a *.rls.test.ts file under tests/.
func checkAuthzCoverage(root, specYMLPath string) []AuditGap {
	data, err := os.ReadFile(specYMLPath)
	if err != nil {
		return nil
	}
	var manifest SpecManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil || manifest.AuthzModel == "" {
		return nil
	}
	roles := extractAuthzRoles(manifest.AuthzModel)
	if len(roles) == 0 {
		return nil
	}

	rlsFiles, _ := filepath.Glob(filepath.Join(root, "tests", "*.rls.test.ts"))
	if len(rlsFiles) == 0 {
		return []AuditGap{{
			Type:     "authz-role-untested",
			Severity: "blocking",
			Description: fmt.Sprintf(
				"no *.rls.test.ts found; %d authz role(s) require RLS tests: %s",
				len(roles), strings.Join(roles, ", "),
			),
			File: filepath.Join(root, "tests"),
			Hint: "run `forge ship test` to generate RLS test stubs",
		}}
	}

	var sb strings.Builder
	for _, f := range rlsFiles {
		b, _ := os.ReadFile(f)
		sb.Write(b)
	}
	content := sb.String()

	var missing []string
	for _, role := range roles {
		if !strings.Contains(content, role) {
			missing = append(missing, role)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return []AuditGap{{
		Type:     "authz-role-untested",
		Severity: "blocking",
		Description: fmt.Sprintf(
			"%d authz role(s) not referenced in RLS tests: %s",
			len(missing), strings.Join(missing, ", "),
		),
		File: rlsFiles[0],
		Hint: "add test cases for each missing role in the *.rls.test.ts file",
	}}
}

// checkEventCoverage verifies each event in spec.yml.events_emitted is asserted
// in tests/<slug>.test.ts.
func checkEventCoverage(root, slug, specYMLPath string) []AuditGap {
	data, err := os.ReadFile(specYMLPath)
	if err != nil {
		return nil
	}
	var manifest SpecManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil || len(manifest.EventsEmitted) == 0 {
		return nil
	}

	testFile := filepath.Join(root, "tests", slug+".test.ts")
	testData, err := os.ReadFile(testFile)
	if err != nil {
		return []AuditGap{{
			Type:     "missing-event-test",
			Severity: "warning",
			Description: fmt.Sprintf(
				"test file %s.test.ts not found; %d event(s) in spec require test assertions",
				slug, len(manifest.EventsEmitted),
			),
			File: testFile,
			Hint: "run `forge ship test` to generate test stubs",
		}}
	}
	content := string(testData)

	var missing []string
	for _, event := range manifest.EventsEmitted {
		if !strings.Contains(content, event) {
			missing = append(missing, event)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return []AuditGap{{
		Type:     "missing-event-test",
		Severity: "warning",
		Description: fmt.Sprintf(
			"%d event(s) from spec.yml not asserted in %s.test.ts: %s",
			len(missing), slug, strings.Join(missing, ", "),
		),
		File: testFile,
		Hint: "add assertions for each missing event in the test file",
	}}
}

// extractAuthzRoles pulls role names from an authz_model string.
// Recognises patterns such as:
//
//	role: admin
//	roles: admin, user, viewer
func extractAuthzRoles(authzModel string) []string {
	if authzModel == "" {
		return nil
	}
	var roles []string
	seen := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(authzModel))
	for scanner.Scan() {
		line := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if strings.HasPrefix(line, "role:") || strings.HasPrefix(line, "roles:") {
			rest := strings.SplitN(line, ":", 2)[1]
			for _, part := range strings.FieldsFunc(rest, func(r rune) bool {
				return r == ',' || r == ' ' || r == '|' || r == '/'
			}) {
				part = strings.TrimSpace(part)
				if part != "" && !seen[part] {
					seen[part] = true
					roles = append(roles, part)
				}
			}
		}
	}
	return roles
}
