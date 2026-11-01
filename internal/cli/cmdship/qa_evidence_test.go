package cmdship

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/agentbridge"
)

const qaTestSpec = "# Spec\n\n- AC-1 user can post\n- AC-2 user can undo\n"

func TestMissingACReferences(t *testing.T) {
	t.Parallel()
	if got := missingACReferences(qaTestSpec, "| AC-1 | ok | PASS |\n| AC-10 | x | PASS |"); len(got) != 1 || got[0] != "AC-2" {
		t.Fatalf("AC-10 must not satisfy AC-2; got %v", got)
	}
	if got := missingACReferences(qaTestSpec, "AC-1 AC-2"); len(got) != 0 {
		t.Fatalf("expected no gaps, got %v", got)
	}
}

func TestMissingManualPlanRoles(t *testing.T) {
	t.Parallel()
	got := missingManualPlanRoles("## Product Owner\n## Security\n")
	if len(got) != 4 {
		t.Fatalf("expected 4 missing roles, got %v", got)
	}
}

func TestRequestQAEvidence_IssuesTurnsAndWritesValidAnswers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "post-undo"
	dir := filepath.Join(root, ".forge", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(qaTestSpec), 0o600); err != nil {
		t.Fatal(err)
	}
	bridge, err := agentbridge.Open(root, agentbridge.DefaultSession)
	if err != nil {
		t.Fatal(err)
	}
	pipe := newLLMPipeAgent(root, bridge)

	answers := map[string]string{
		"ship:qa:manual-plan":      "## Product Owner\n## Business Analyst\n## Quality Engineer\n## Security Reviewer\n## DevOps / SRE\n## Compliance / CPO\n",
		"ship:qa:testing-pipeline": "## Local\n## Pre-push\n## Staging\n## Production\n",
		"ship:qa:report":           "| AC-1 | e | PASS |\n| AC-2 | e | PASS |\n",
	}
	for i := 0; i < 3; i++ {
		op, problems := requestQAEvidence(root, slug, "post undo", pipe)
		if op == "" {
			t.Fatalf("round %d: expected a pending turn, problems=%v", i, problems)
		}
		if _, err := bridge.Fulfil(answers[op]); err != nil {
			t.Fatalf("Fulfil(%s): %v", op, err)
		}
	}
	op, problems := requestQAEvidence(root, slug, "post undo", pipe)
	if op != "" || len(problems) != 0 {
		t.Fatalf("expected all evidence accepted, op=%q problems=%v", op, problems)
	}
	for _, f := range []string{"manual-test-plan.md", "testing-pipeline.md", "qa-report.md"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("%s not written: %v", f, err)
		}
	}
}

func TestRequestQAEvidence_RejectsIncompleteAnswer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "post-undo"
	bridge, err := agentbridge.Open(root, agentbridge.DefaultSession)
	if err != nil {
		t.Fatal(err)
	}
	pipe := newLLMPipeAgent(root, bridge)
	if op, _ := requestQAEvidence(root, slug, "post undo", pipe); op != "ship:qa:manual-plan" {
		t.Fatalf("first turn = %q", op)
	}
	if _, err := bridge.Fulfil("## Product Owner only"); err != nil {
		t.Fatal(err)
	}
	_, problems := requestQAEvidence(root, slug, "post undo", pipe)
	if len(problems) == 0 || !strings.Contains(problems[0], "manual-test-plan.md") {
		t.Fatalf("expected a rejection for manual-test-plan.md, got %v", problems)
	}
	if _, err := os.Stat(filepath.Join(root, ".forge", "specs", slug, "manual-test-plan.md")); err == nil {
		t.Fatal("invalid answer must not be written")
	}
}

func TestPickIntegrationDir_PrefersTestsDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fallback := filepath.Join(root, "src", "test")
	if got := pickIntegrationDir(root, fallback); got != fallback {
		t.Fatalf("no tests/ dir: want fallback, got %s", got)
	}
	if err := os.MkdirAll(filepath.Join(root, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := pickIntegrationDir(root, fallback); got != filepath.Join(root, "tests") {
		t.Fatalf("tests/ exists: want it, got %s", got)
	}
}
