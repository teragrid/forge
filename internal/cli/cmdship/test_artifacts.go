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

// Package cmdship — G-006: four named test artifacts produced at the Test checkpoint.
//
// The four artifacts written to tests/<slug>/ are:
//
//  1. <slug>.test.ts        — unit test stubs (TypeScript; intentionally failing)
//  2. <slug>.integration.test.ts — integration test stubs
//  3. <slug>.rls.test.ts    — Row-Level Security test stubs
//  4. <slug>.scan.baseline.json  — scan baseline (empty allowlist)
//
// Tests are intentionally failing ("red") so that the Code checkpoint can verify
// TDD compliance. If all stubs pass at generation time the Test checkpoint aborts
// with ErrTestsGreenAtStubStage (FORGE-3201).
package cmdship

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/teragrid/forge/internal/errcode"
)

// ErrTestsGreenAtStubStage is returned when generated test stubs already pass.
// This signals a TDD violation: tests must be RED at stub creation time.
var ErrTestsGreenAtStubStage = errcode.Register(errcode.Code(3202),
	"test stubs already passing — tests must be failing (red) at stub stage (TDD requirement)")

// TestArtifactPaths holds the paths of the four G-006 test artifacts for a slug.
type TestArtifactPaths struct {
	UnitTest        string // tests/<slug>.test.ts
	IntegrationTest string // tests/<slug>.integration.test.ts
	RLSTest         string // tests/<slug>.rls.test.ts
	ScanBaseline    string // tests/<slug>.scan.baseline.json
}

// writeTestArtifacts generates the four named test artifacts for G-006.
//
// It creates the files in <root>/tests/<slug>/ and returns the paths.
// An LLMPipe may be nil — in that case deterministic stubs are written.
// The function never returns an error for file-write failures; a missing
// test artifact is surfaced as a warning in checkTest.
func writeTestArtifacts(root, slug, feature, specMarkdown string, pipe *LLMPipe) TestArtifactPaths {
	testsDir := filepath.Join(root, "tests")
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		return TestArtifactPaths{}
	}

	paths := TestArtifactPaths{
		UnitTest:        filepath.Join(testsDir, slug+".test.ts"),
		IntegrationTest: filepath.Join(testsDir, slug+".integration.test.ts"),
		RLSTest:         filepath.Join(testsDir, slug+".rls.test.ts"),
		ScanBaseline:    filepath.Join(testsDir, slug+".scan.baseline.json"),
	}

	// Generate via LLM when available; fall back to static stubs.
	unitContent := unitTestStub(slug, feature)
	integContent := integrationTestStub(slug, feature)
	rlsContent := rlsTestStub(slug, feature)

	if pipe != nil {
		ctx := ""
		if specMarkdown != "" {
			ctx = "Feature spec:\n" + specMarkdown + "\n\n"
		}

		if gen, err := pipe.Invoke("ship:test:unit", "",
			"You are a senior TypeScript QA engineer writing failing unit test stubs for TDD. "+
				"Tests MUST compile but MUST fail at runtime (use t.fail() or expect(false).toBe(true)). "+
				"Use Jest + supertest. Each test covers exactly one acceptance criterion.",
			ctx+"Generate failing unit test stubs for feature: "+feature, 2000); err == nil && gen != "" {
			unitContent = gen
		}

		if gen, err := pipe.Invoke("ship:test:integration", "",
			"You are writing failing integration test stubs (Jest + supertest) for TDD. "+
				"Tests MUST fail. Cover: happy path, auth, and error scenarios.",
			ctx+"Generate failing integration test stubs for feature: "+feature, 2000); err == nil && gen != "" {
			integContent = gen
		}

		if gen, err := pipe.Invoke("ship:test:rls", "",
			"You are writing Row-Level Security test stubs for Supabase/PostgreSQL. "+
				"Tests MUST fail. Verify that: (1) tenant-A cannot read tenant-B rows, "+
				"(2) service role can read all rows, (3) anon role is denied.",
			ctx+"Generate failing RLS test stubs for feature: "+feature, 1500); err == nil && gen != "" {
			rlsContent = gen
		}
	}

	_ = os.WriteFile(paths.UnitTest, []byte(unitContent), 0o600)
	_ = os.WriteFile(paths.IntegrationTest, []byte(integContent), 0o600)
	_ = os.WriteFile(paths.RLSTest, []byte(rlsContent), 0o600)

	baseline := scanBaseline(slug, feature)
	if data, err := json.MarshalIndent(baseline, "", "  "); err == nil {
		_ = os.WriteFile(paths.ScanBaseline, data, 0o600)
	}

	return paths
}

// allTestArtifactsExist returns true when all four G-006 artifacts are present.
func allTestArtifactsExist(root, slug string) bool { //nolint:unused // used in ship dry-run gate
	testsDir := filepath.Join(root, "tests")
	for _, name := range []string{
		slug + ".test.ts",
		slug + ".integration.test.ts",
		slug + ".rls.test.ts",
		slug + ".scan.baseline.json",
	} {
		if _, err := os.Stat(filepath.Join(testsDir, name)); err != nil {
			return false
		}
	}
	return true
}

// missingTestArtifacts lists the artifact filenames that are absent.
func missingTestArtifacts(root, slug string) []string {
	testsDir := filepath.Join(root, "tests")
	var missing []string
	for _, name := range []string{
		slug + ".test.ts",
		slug + ".integration.test.ts",
		slug + ".rls.test.ts",
		slug + ".scan.baseline.json",
	} {
		if _, err := os.Stat(filepath.Join(testsDir, name)); err != nil {
			missing = append(missing, name)
		}
	}
	return missing
}

// ── Static stub generators ────────────────────────────────────────────────────

func unitTestStub(slug, feature string) string {
	return fmt.Sprintf(`// AUTO-GENERATED by forge ship test — G-006 unit test stubs
// Feature: %s
// Status: RED (intentionally failing — complete implementation in forge ship code)
//
// Edit this file to add real assertions once the feature is implemented.

import { describe, it, expect, beforeAll, afterAll } from '@jest/globals';

describe('%s — unit', () => {
  it('should be implemented (placeholder — will fail until feature is complete)', () => {
    // TODO: replace with real assertion after forge ship code
    expect(false).toBe(true); // intentionally failing
  });

  it('given valid input, when processed, then expected output returned', () => {
    // TODO: implement
    expect(false).toBe(true);
  });

  it('given invalid input, when processed, then validation error returned', () => {
    // TODO: implement
    expect(false).toBe(true);
  });
});
`, feature, slug)
}

func integrationTestStub(slug, feature string) string {
	return fmt.Sprintf(`// AUTO-GENERATED by forge ship test — G-006 integration test stubs
// Feature: %s
// Status: RED (intentionally failing)

import { describe, it, expect, beforeAll, afterAll } from '@jest/globals';
import request from 'supertest';
import { app } from '../src/app';

describe('%s — integration', () => {
  let server: ReturnType<typeof app.listen>;

  beforeAll(() => { server = app.listen(0); });
  afterAll(() => server.close());

  it('POST /api/%s — happy path returns 200', async () => {
    // TODO: replace with real test after forge ship code
    const res = await request(server).post('/api/%s').send({});
    expect(res.status).toBe(200); // will fail until implementation is complete
  });

  it('POST /api/%s — unauthenticated returns 401', async () => {
    const res = await request(server).post('/api/%s').send({});
    expect(res.status).toBe(401);
  });

  it('POST /api/%s — invalid payload returns 422', async () => {
    const res = await request(server).post('/api/%s').send({ invalid: true });
    expect(res.status).toBe(422);
  });
});
`, feature, slug, slug, slug, slug, slug, slug, slug)
}

func rlsTestStub(slug, feature string) string {
	return fmt.Sprintf(`// AUTO-GENERATED by forge ship test — G-006 RLS test stubs
// Feature: %s
// Status: RED (intentionally failing)
//
// These tests verify Row-Level Security policies for the feature.
// Run against a local Supabase instance: supabase start

import { describe, it, expect, beforeAll, afterAll } from '@jest/globals';
import { createClient } from '@supabase/supabase-js';

const SUPABASE_URL = process.env.SUPABASE_URL ?? 'http://localhost:54321';
const ANON_KEY    = process.env.SUPABASE_ANON_KEY ?? '';
const SERVICE_KEY = process.env.SUPABASE_SERVICE_KEY ?? '';

describe('%s — RLS policies', () => {
  const anon    = createClient(SUPABASE_URL, ANON_KEY);
  const service = createClient(SUPABASE_URL, SERVICE_KEY);

  it('anon role cannot read %s rows', async () => {
    // TODO: replace table name + assertion after schema migration
    const { data, error } = await anon.from('%s').select('*');
    expect(error).not.toBeNull(); // RLS should block anon reads
  });

  it('service role can read all %s rows', async () => {
    const { error } = await service.from('%s').select('*');
    expect(error).toBeNull();
  });

  it('tenant-A cannot read tenant-B rows', async () => {
    // TODO: implement with two tenant JWT fixtures
    expect(false).toBe(true); // placeholder
  });
});
`, feature, slug, slug, slug, slug, slug)
}

// ScanBaseline is the G-006 scan baseline JSON — an empty allowlist that
// downstream scan runs diff against to detect regressions.
type ScanBaseline struct {
	SchemaVersion int              `json:"schema_version"`
	Feature       string           `json:"feature"`
	Slug          string           `json:"slug"`
	Families      []string         `json:"families"`
	Allowlist     []ScanAllowEntry `json:"allowlist"`
	GeneratedAt   string           `json:"generated_at"`
}

// ScanAllowEntry represents a single allowlisted finding.
type ScanAllowEntry struct {
	Rule     string `json:"rule"`
	File     string `json:"file"`
	Reason   string `json:"reason"`
	ReviewBy string `json:"review_by"` // RFC-3339 date; G-069 expiry gate
}

func scanBaseline(slug, feature string) ScanBaseline {
	return ScanBaseline{
		SchemaVersion: 1,
		Feature:       feature,
		Slug:          slug,
		Families:      []string{"secrets", "sca", "sast", "license", "iac", "container", "api", "supply-chain"},
		Allowlist:     []ScanAllowEntry{},
		GeneratedAt:   "",
	}
}
