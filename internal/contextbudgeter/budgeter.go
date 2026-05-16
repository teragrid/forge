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

// Package contextbudgeter implements G-041: token-budget enforcement for
// prompt construction.
//
// CheckBudget returns an error if the estimated token count of the prompt
// exceeds maxTokens. EstimateTokens provides a fast heuristic count (≈1 token
// per 4 chars) without requiring a live tokenizer.
package contextbudgeter

import (
	"fmt"
)

// EstimateTokens returns a fast heuristic estimate of the token count for a
// given text. It uses the common rule-of-thumb of ~4 characters per token,
// which is accurate to ±15% for English prose and code.
func EstimateTokens(text string) int {
	chars := len([]rune(text))
	tokens := chars / 4
	if chars%4 != 0 {
		tokens++
	}
	return tokens
}

// CheckBudget returns an error if the estimated token count of prompt exceeds
// maxTokens. Use maxTokens == 0 to skip the check.
func CheckBudget(prompt string, maxTokens int) error {
	if maxTokens <= 0 {
		return nil
	}
	est := EstimateTokens(prompt)
	if est > maxTokens {
		return fmt.Errorf("prompt exceeds token budget: estimated %d tokens > limit %d", est, maxTokens)
	}
	return nil
}
