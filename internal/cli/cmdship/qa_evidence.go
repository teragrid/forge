package cmdship

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// qa_evidence.go — qa-verify turns for the host agent (--agent-mode).
//
// In agent mode forge cannot generate the qa-verify evidence itself: the host
// agent is the QA agent. Before this file existed no qa-verify turn was ever
// issued, so the three files the qa-verify gates read were only discoverable
// by failing them one at a time. Each file is now requested as its own turn,
// validated against the same rules the gates apply, and written verbatim.

// manualPlanRoles are the six role headings manualTestPlanGate requires.
var manualPlanRoles = []struct{ keyword, role string }{
	{"product owner", "Product Owner (UAT)"},
	{"business analyst", "Business Analyst"},
	{"quality engineer", "Quality Engineer"},
	{"security", "Security Reviewer"},
	{"devops", "DevOps / SRE"},
	{"compliance", "Compliance / CPO"},
}

// missingManualPlanRoles reports the role sections absent from content.
func missingManualPlanRoles(content string) []string {
	lower := strings.ToLower(content)
	var missing []string
	for _, s := range manualPlanRoles {
		if !strings.Contains(lower, s.keyword) {
			missing = append(missing, s.role)
		}
	}
	return missing
}

// specACKeys lists the acceptance-criteria keys qaCoverageGate looks for in
// qa-report.md: the AC-n id when a line has one, else its 30-char prefix.
func specACKeys(specMD string) (keys []string, byID []bool) {
	for _, line := range strings.Split(specMD, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "Given ") && !strings.HasPrefix(trimmed, "- AC-") {
			continue
		}
		if id := acIDPattern.FindString(trimmed); id != "" {
			keys, byID = append(keys, id), append(byID, true)
			continue
		}
		if len(trimmed) > 30 {
			trimmed = trimmed[:30]
		}
		keys, byID = append(keys, trimmed), append(byID, false)
	}
	return keys, byID
}

// missingACReferences reports the spec acceptance criteria qaReport does not cite.
func missingACReferences(specMD, qaReport string) []string {
	keys, byID := specACKeys(specMD)
	var missing []string
	for i, k := range keys {
		if byID[i] {
			if !containsACID(qaReport, k) {
				missing = append(missing, k)
			}
		} else if !strings.Contains(qaReport, k) {
			missing = append(missing, k)
		}
	}
	return missing
}

// qaEvidenceTurn describes one evidence file requested from the host agent.
type qaEvidenceTurn struct {
	op       string
	file     string
	system   string
	validate func(content, specMD string) []string // returns what is still missing
}

func qaEvidenceTurns() []qaEvidenceTurn {
	return []qaEvidenceTurn{
		{
			op:   "ship:qa:manual-plan",
			file: "manual-test-plan.md",
			system: "You are the QA lead for this feature. Write manual-test-plan.md as Markdown with one " +
				"heading for each role: Product Owner (UAT), Business Analyst, Quality Engineer, Security Reviewer, " +
				"DevOps / SRE, Compliance / CPO. Each section holds concrete numbered scenarios with inputs and expected results.",
			validate: func(c, _ string) []string { return missingManualPlanRoles(c) },
		},
		{
			op:   "ship:qa:testing-pipeline",
			file: "testing-pipeline.md",
			system: "Write testing-pipeline.md as Markdown documenting evidence for all four testing stages of this " +
				"change: local, pre-push, staging, production. Give each stage its own heading with the commands run and their results.",
			validate: func(c, _ string) []string { return missingTestingPipelineStages(c) },
		},
		{
			op:   "ship:qa:report",
			file: "qa-report.md",
			system: "Write qa-report.md as Markdown with a coverage table that cites EVERY acceptance criterion of the " +
				"spec by its AC-n id (or its leading words when unnumbered), with evidence and a PASS/FAIL verdict for each.",
			validate: func(c, spec string) []string { return missingACReferences(spec, c) },
		},
	}
}

// requestQAEvidence asks the host agent for each missing or incomplete
// qa-verify evidence file. It returns paused=true (with the pending operation)
// when a turn is waiting on the host agent, and otherwise a list of problems
// for answers that came back but do not satisfy the gates. Nothing invalid is
// written to disk.
func requestQAEvidence(root, slug, description string, pipe *LLMPipe) (pausedOp string, problems []string) {
	dir := filepath.Join(root, ".forge", "specs", slug)
	specData, _ := os.ReadFile(filepath.Join(dir, "spec.md"))
	specMD := string(specData)

	for _, t := range qaEvidenceTurns() {
		path := filepath.Join(dir, t.file)
		if existing, err := os.ReadFile(path); err == nil && len(t.validate(string(existing), specMD)) == 0 {
			continue
		}
		user := fmt.Sprintf("Feature: %s\n\nSpec:\n%s\n\n%s", description, specMD, qaVerifyChecklist(slug))
		content, err := pipe.Invoke(t.op, "", t.system, user, 4000)
		if IsAgentTurn(err) {
			return t.op, problems
		}
		if err != nil || strings.TrimSpace(content) == "" {
			problems = append(problems, t.file+": no answer")
			continue
		}
		if missing := t.validate(content, specMD); len(missing) > 0 {
			problems = append(problems, fmt.Sprintf("%s: answer missing %s", t.file, strings.Join(missing, ", ")))
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			problems = append(problems, t.file+": "+err.Error())
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			problems = append(problems, t.file+": "+err.Error())
		}
	}
	return "", problems
}
