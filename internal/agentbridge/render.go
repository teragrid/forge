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

// render.go — how a pending turn is presented to the host agent.
//
// The block is written for a reader that has no memory of the previous turn:
// a chat window whose context was compacted, or a fresh `forge agent prompt`
// in a new session. Everything needed to answer — the prompts, the budget,
// and the exact command to submit the answer — is in the block itself.
package agentbridge

import (
	"fmt"
	"strings"
)

// Fence is the delimiter that brackets the prompt payload. It is deliberately
// longer than a normal Markdown fence so that fenced code inside the prompt
// (which is common — specs are full of Gherkin and YAML blocks) can never
// terminate it early.
const Fence = "````````"

// RenderTurn formats a pending turn as the block the host agent reads.
//
// session is included in the submit command so a user driving more than one
// feature at once cannot answer the wrong question.
func RenderTurn(t Turn, session string) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("═══ FORGE AGENT TURN ════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("  operation   %s\n", t.Operation))
	if t.Checkpoint != "" {
		sb.WriteString(fmt.Sprintf("  checkpoint  %s\n", t.Checkpoint))
	}
	sb.WriteString(fmt.Sprintf("  budget      ~%d output tokens\n", t.MaxTokens))
	sb.WriteString(fmt.Sprintf("  turn        %s (%s)\n", t.Ordinal, t.Hash[:12]))
	sb.WriteString("═════════════════════════════════════════════════════════════════════\n\n")

	sb.WriteString("You are standing in for the LLM provider forge would normally call.\n")
	sb.WriteString("Answer the prompt below and submit the answer back to forge.\n\n")

	sb.WriteString("── SYSTEM PROMPT " + strings.Repeat("─", 52) + "\n")
	sb.WriteString(Fence + "\n")
	sb.WriteString(strings.TrimRight(t.System, "\n"))
	sb.WriteString("\n" + Fence + "\n\n")

	sb.WriteString("── USER PROMPT " + strings.Repeat("─", 54) + "\n")
	sb.WriteString(Fence + "\n")
	sb.WriteString(strings.TrimRight(t.User, "\n"))
	sb.WriteString("\n" + Fence + "\n\n")

	sb.WriteString(renderContract(session))
	return sb.String()
}

// renderContract is the fixed instruction block appended to every turn. It is
// the same every time on purpose: the host agent should learn one loop, not a
// per-operation dialect.
func renderContract(session string) string {
	sessionFlag := ""
	if session != "" && session != DefaultSession {
		sessionFlag = " --session " + session
	}
	var sb strings.Builder
	sb.WriteString("── HOW TO ANSWER " + strings.Repeat("─", 52) + "\n")
	sb.WriteString("1. Produce ONLY the artefact the prompt asks for.\n")
	sb.WriteString("   No preamble, no \"Here is the spec you asked for\", no closing remarks.\n")
	sb.WriteString("   Forge validates the answer structurally and will reject a preamble.\n")
	sb.WriteString("2. Write it to a file, then submit it:\n\n")
	sb.WriteString("     forge agent submit --file <path>" + sessionFlag + "\n\n")
	sb.WriteString("   or pipe it on stdin:\n\n")
	sb.WriteString("     forge agent submit -" + sessionFlag + " < answer.md\n\n")
	sb.WriteString("3. Continue the pipeline:\n\n")
	sb.WriteString("     forge ship --agent-mode" + sessionFlag + "\n\n")
	sb.WriteString("Forge replays your answer, runs the deterministic gates against it,\n")
	sb.WriteString("and either advances to the next checkpoint or hands you the next turn.\n")
	sb.WriteString("Repeat until the run reports ready.\n\n")
	sb.WriteString("You do NOT decide whether a checkpoint passed — forge does.\n")
	sb.WriteString("Never report a gate as green that forge has not reported as green.\n")
	return sb.String()
}

// RenderStatus formats bridge stats for `forge agent status`.
func RenderStatus(s Stats) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("session      %s\n", s.Session.Name))
	if s.Session.Feature != "" {
		sb.WriteString(fmt.Sprintf("feature      %s\n", s.Session.Feature))
	}
	if s.Session.Slug != "" {
		sb.WriteString(fmt.Sprintf("slug         %s\n", s.Session.Slug))
	}
	sb.WriteString(fmt.Sprintf("turns filled %d\n", s.Session.TurnsFilled))
	sb.WriteString(fmt.Sprintf("responses    %d recorded\n", s.Responses))
	if s.Drifted > 0 {
		sb.WriteString(fmt.Sprintf("drift        %d prompt(s) replayed by position, not content\n", s.Drifted))
	}
	if s.Pending != nil {
		sb.WriteString(fmt.Sprintf("pending      %s — run `forge agent prompt` to see it\n", s.Pending.Ordinal))
	} else {
		sb.WriteString("pending      none — run `forge ship --agent-mode` to continue\n")
	}
	return sb.String()
}
