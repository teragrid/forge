// test_review.go — 3-role parallel test debate. RFC-005 §6.4.
// Three reviewer personas (QA Architect, Security Tester, Reliability Tester)
// run concurrently via LLM and their outputs are synthesised into a single
// TestReviewResult. When tier is T0, the debate is skipped in favour of a
// single QA Architect pass (config: ship.test_debate_threshold=T0).
package cmdship

import (
	"context"
	"sync"
)

// TestReviewResult holds the synthesised output of the 3-role test debate.
type TestReviewResult struct {
	// QAArchitectFeedback covers D1–D9 dimension coverage gaps.
	QAArchitectFeedback string
	// SecurityTesterFeedback focuses on D6 (AuthZ) + D3 (Negative).
	SecurityTesterFeedback string
	// ReliabilityTesterFeedback focuses on D4 (Idempotency) + D5 (Concurrency) + D8 (DataAccuracy).
	ReliabilityTesterFeedback string
	// Synthesis is the merged consensus of all three reviewers.
	Synthesis string
	// SkippedDebate is true when the 3-role debate was skipped (T0 tier or nil pipe).
	SkippedDebate bool
}

// RunTestDebate runs a 3-role parallel test review and synthesises the results.
//
//   - ctx:      cancellation context (callers may impose a deadline)
//   - slug:     feature slug (used for LLM cache keying)
//   - feature:  human-readable feature description
//   - testPlan: markdown test plan / stub contents to review
//   - fw:       detected test framework context
//   - pipe:     LLM pipe — when nil, SkippedDebate=true and empty feedback returned
//   - tier:     "T0" | "T1" | "T2" — T0 skips 3-role debate, single QA pass only
//
// RFC-005 §6.4.
func RunTestDebate(ctx context.Context, slug, feature, testPlan string, fw TestFrameworkContext, pipe *LLMPipe, tier string) TestReviewResult {
	if pipe == nil {
		return TestReviewResult{SkippedDebate: true}
	}

	// T0 tier: skip full debate, run single QA Architect pass only.
	if tier == "T0" {
		feedback := singleQAPass(ctx, slug, feature, testPlan, fw, pipe)
		return TestReviewResult{
			QAArchitectFeedback: feedback,
			Synthesis:           feedback,
			SkippedDebate:       true,
		}
	}

	// Full 3-role parallel debate.
	var (
		qaFeedback  string
		secFeedback string
		relFeedback string
		wg          sync.WaitGroup
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		qaFeedback = llmReview(ctx, pipe, "ship:review:qa",
			"You are a QA Architect with 20+ years experience. Review the test plan for all 9 dimensions: "+
				"D1 Happy Path, D2 Boundary, D3 Negative, D4 Idempotency, D5 Concurrency, "+
				"D6 AuthZ/Cross-Tenant, D7 Regression Guard, D8 Data Accuracy, D9 False-Positive Guard. "+
				"List gaps per dimension. Be concise.",
			feature, testPlan, fw)
	}()
	go func() {
		defer wg.Done()
		secFeedback = llmReview(ctx, pipe, "ship:review:security",
			"You are a Security Tester specialising in authorization failures and injection. "+
				"Focus ONLY on D6 (Cross-tenant AuthZ, path traversal, privilege escalation) and "+
				"D3 (Negative: invalid inputs, boundary violations, injection payloads). "+
				"Flag any missing security test cases.",
			feature, testPlan, fw)
	}()
	go func() {
		defer wg.Done()
		relFeedback = llmReview(ctx, pipe, "ship:review:reliability",
			"You are a Reliability Tester specialising in distributed systems. "+
				"Focus ONLY on D4 (Idempotency: same-operation-twice, webhook replay), "+
				"D5 (Concurrency: data races, two writers), and "+
				"D8 (Data Accuracy: real inserts → query back → numeric/temporal correctness). "+
				"Flag any missing reliability test cases.",
			feature, testPlan, fw)
	}()
	wg.Wait()

	// Synthesis: QA Architect persona, given all three reviews.
	synthesisPrompt := "Given three expert test reviews below, produce a single prioritised list of test gaps " +
		"ordered by severity (BLOCK > WARNING > ADVISORY).\n\n" +
		"QA Architect:\n" + qaFeedback + "\n\n" +
		"Security Tester:\n" + secFeedback + "\n\n" +
		"Reliability Tester:\n" + relFeedback
	synthesis, _ := pipe.Invoke("ship:review:synthesis", slug,
		"You are a QA Architect producing a synthesis of three test reviews.",
		synthesisPrompt, 1200)

	return TestReviewResult{
		QAArchitectFeedback:       qaFeedback,
		SecurityTesterFeedback:    secFeedback,
		ReliabilityTesterFeedback: relFeedback,
		Synthesis:                 synthesis,
	}
}

// singleQAPass runs only the QA Architect reviewer (used for T0 tier).
func singleQAPass(_ context.Context, _ string, feature, testPlan string, fw TestFrameworkContext, pipe *LLMPipe) string {
	return llmReview(context.TODO(), pipe, "ship:review:qa",
		"You are a QA Architect. Review the test plan for all 9 test quality dimensions. List gaps.",
		feature, testPlan, fw)
}

// llmReview invokes a single reviewer persona via the LLM pipe.
func llmReview(_ context.Context, pipe *LLMPipe, tag, sysPrompt, feature, testPlan string, fw TestFrameworkContext) string {
	if pipe == nil {
		return ""
	}
	fwNote := ""
	if fw.Language != "" {
		fwNote = " Language: " + fw.Language + ". Test runner: " + fw.TestRunner + "."
	}
	userMsg := "Feature: " + feature + fwNote + "\n\nTest plan:\n" + testPlan
	out, _ := pipe.Invoke(tag, "", sysPrompt, userMsg, 800)
	return out
}
