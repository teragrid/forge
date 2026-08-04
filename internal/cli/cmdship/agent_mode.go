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

// agent_mode.go — `forge ship --agent-mode`: the CLI half of the two-plane
// split (see internal/agentbridge for the boundary itself).
//
// The pipeline runs unchanged. The only difference is what happens when a
// checkpoint needs a model: instead of calling a provider, forge stops, hands
// the compiled prompt to whatever AI chat the user is already in, and exits
// with ExitAgentTurn. The user's agent answers, submits, and re-runs. Gates,
// artefact validation, and ledgers are identical either way — the host agent
// supplies text, never verdicts.
package cmdship

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/agentbridge"
	"github.com/teragrid/forge/internal/errcode"
)

// agentTurnPayload is the --json shape of a paused run. It carries the full
// prompt text rather than a path, because the most common consumer is an MCP
// or subprocess caller that reads stdout and never touches the filesystem.
type agentTurnPayload struct {
	Status     string `json:"status"` // always "agent_turn_required"
	Session    string `json:"session"`
	Operation  string `json:"operation"`
	Checkpoint string `json:"checkpoint,omitempty"`
	Ordinal    string `json:"ordinal"`
	Hash       string `json:"hash"`
	MaxTokens  int    `json:"max_tokens"`
	System     string `json:"system"`
	User       string `json:"user"`
	SubmitCmd  string `json:"submit_cmd"`
	ResumeCmd  string `json:"resume_cmd"`
	ExitCode   int    `json:"exit_code"`
}

// renderAgentTurn prints the pending turn and returns the sentinel error that
// main() maps to ExitAgentTurn.
func renderAgentTurn(cmd *cobra.Command, t agentbridge.Turn, session string, asJSON bool) error {
	sessionFlag := ""
	if session != "" && session != agentbridge.DefaultSession {
		sessionFlag = " --session " + session
	}
	if asJSON {
		payload := agentTurnPayload{
			Status:     "agent_turn_required",
			Session:    session,
			Operation:  t.Operation,
			Checkpoint: t.Checkpoint,
			Ordinal:    t.Ordinal,
			Hash:       t.Hash,
			MaxTokens:  t.MaxTokens,
			System:     t.System,
			User:       t.User,
			SubmitCmd:  "forge agent submit --file <path>" + sessionFlag,
			ResumeCmd:  "forge ship --agent-mode" + sessionFlag,
			ExitCode:   ExitAgentTurn,
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			return err
		}
	} else {
		fmt.Fprint(cmd.OutOrStdout(), agentbridge.RenderTurn(t, session))
	}
	return errcode.New(ErrAgentTurn,
		"pipeline paused at "+t.Operation+" — answer the turn above, then re-run", nil)
}

// IsAgentTurn reports whether err is a paused agent-mode run rather than a
// real failure. Used by main() to select ExitAgentTurn over exit 1.
func IsAgentTurn(err error) bool {
	var e *errcode.Error
	if !errors.As(err, &e) {
		return false
	}
	return e.Code == ErrAgentTurn
}
