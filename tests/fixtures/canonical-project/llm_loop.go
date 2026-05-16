package main

// Canonical fixture: triggers cost scanner (llm-call-in-loop rule).
// The for-range loop and completion() call appear on the same line, which
// matches "llm-call-in-loop": loop keyword (for) + LLM call (completion).

// LLMLoopExample demonstrates the cost anti-pattern: a completion() call
// inside a for loop on a single line triggers the scanner.
func LLMLoopExample(items []string) []string {
	var results []string
	// cost: llm-call-in-loop — for + completion on the same line
	for i := range items { results = append(results, completion(items[i])) }
	return results
}

// completion is a stub representing an LLM completion call.
func completion(prompt string) string { return prompt }
