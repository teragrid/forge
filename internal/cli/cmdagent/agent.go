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

// Package cmdagent implements `forge agent` — the host-agent side of
// `forge ship --agent-mode`.
//
// `forge ship --agent-mode` pauses whenever the pipeline needs reasoning and
// hands the prompt to whatever AI chat the user is already in. `forge agent`
// is how that chat answers:
//
//	forge agent status            what run is in flight, what is owed
//	forge agent prompt            re-print the pending turn
//	forge agent submit --file X   answer it
//	forge agent loop              the driver protocol, for pasting into a chat
//	forge agent reset             discard recorded answers and start over
//
// The split exists so that no part of the deterministic plane — gates,
// artefact validation, ledgers — moves into the model's head. `forge agent`
// carries text in one direction only; every verdict still comes from Go.
package cmdagent

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/agentbridge"
	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 6900..6999).
var (
	ErrAgentFailed = errcode.RegisterWithRemedy(errcode.Code(6900),
		"agent bridge operation failed",
		"forge agent status  # check which session is in flight")
	ErrNoPending = errcode.RegisterWithRemedy(errcode.Code(6901),
		"no pending agent turn",
		"forge ship --agent-mode  # run the pipeline until it needs a turn")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb: "agent",
		Summary: "Drive forge ship from your own AI chat — no API key. " +
			"Answer the prompts forge would otherwise have sent to a provider.",
		Inputs: []string{
			"status              — session, recorded answers, pending turn",
			"prompt              — re-print the pending turn",
			"submit --file <p>   — answer the pending turn from a file",
			"submit -            — answer the pending turn from stdin",
			"loop                — print the driver protocol for a chat window",
			"reset               — discard recorded answers for the session",
			"sessions            — list agent-mode sessions in this project",
			"--session <name>    — which session to act on (default: default)",
			"--root <path>       — project root (default: cwd)",
			"--json / -j         — machine-readable output",
		},
		Outputs: []string{
			"stdout: turn prompt, status, or the driver protocol",
			".forge/agent/<session>/pending.json",
			".forge/agent/<session>/responses.jsonl",
			".forge/agent/<session>/session.json",
		},
		SideEffects: []string{
			"Writes recorded host-agent answers under .forge/agent/<session>/",
		},
		GatesTouched: []string{},
	})
}

// New returns the top-level `forge agent` cobra command.
func New() *cobra.Command {
	var (
		root    string
		session string
		asJSON  bool
	)

	openBridge := func() (*agentbridge.Bridge, error) {
		r := root
		if r == "" {
			var err error
			if r, err = os.Getwd(); err != nil {
				return nil, errcode.New(ErrAgentFailed, "getwd", err)
			}
		}
		b, err := agentbridge.Open(r, session)
		if err != nil {
			return nil, errcode.New(ErrAgentFailed, "open agent bridge", err)
		}
		return b, nil
	}

	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Drive forge ship from your own AI chat — no API key required.",
		Long: `forge agent is the host-agent half of ` + "`forge ship --agent-mode`" + `.

Forge splits into two planes. The deterministic plane — checkpoints, gates,
artefact validation, scan, lint, ledgers — always runs locally in Go and needs
no network. Only the reasoning plane needs a model. Agent mode points that half
at the AI chat you are already paying for instead of at a provider API key.

The loop:

  1. forge ship --agent-mode          runs until it needs reasoning, then
                                      prints a prompt and exits 78
  2. <your AI answers the prompt>
  3. forge agent submit --file out.md records the answer
  4. forge ship --agent-mode          replays it, runs the gates, continues

Repeat until the run reports ready. Run ` + "`forge agent loop`" + ` to get this
protocol in a form you can paste into a chat window.

Your AI supplies text where a provider would have. It never decides whether a
checkpoint passed — forge does, exactly as it does with an API key.`,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}

	cmd.PersistentFlags().StringVarP(&root, "root", "r", "", "project root (default: cwd)")
	cmd.PersistentFlags().StringVar(&session, "session", agentbridge.DefaultSession,
		"agent-mode session name")
	cmd.PersistentFlags().BoolVarP(&asJSON, "json", "j", false, "machine-readable JSON output")

	// ── status ────────────────────────────────────────────────────────────────
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show the in-flight agent-mode run and whether a turn is owed.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			b, err := openBridge()
			if err != nil {
				return err
			}
			stats := b.Stats()
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), stats)
			}
			fmt.Fprint(cmd.OutOrStdout(), agentbridge.RenderStatus(stats))
			return nil
		},
	}

	// ── prompt ────────────────────────────────────────────────────────────────
	promptCmd := &cobra.Command{
		Use:   "prompt",
		Short: "Re-print the pending turn (safe to run as often as you like).",
		Long: `Prints the prompt forge is waiting on.

This is idempotent and holds no state, which is the point: a chat window whose
context was compacted can recover the full question — prompts, budget, and the
submit command — without needing to remember anything from the previous turn.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			b, err := openBridge()
			if err != nil {
				return err
			}
			turn, ok := b.Pending()
			if !ok {
				if asJSON {
					return writeJSON(cmd.OutOrStdout(), map[string]any{"pending": nil})
				}
				return errcode.New(ErrNoPending,
					"nothing to answer in session "+b.SessionName(), nil)
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), turn)
			}
			fmt.Fprint(cmd.OutOrStdout(), agentbridge.RenderTurn(turn, b.SessionName()))
			return nil
		},
	}

	// ── submit ────────────────────────────────────────────────────────────────
	var fromFile string
	submitCmd := &cobra.Command{
		Use:   "submit [-]",
		Short: "Answer the pending turn from a file or stdin.",
		Long: `Records your AI's answer to the pending turn.

The answer is stored, not applied: forge does not write it anywhere in the
project. The next ` + "`forge ship --agent-mode`" + ` replays it into the pipeline at
exactly the point it was asked for, and the same validation that runs on a
provider response runs on it — preamble stripping, truncation detection, and
the checkpoint's own gates. An answer that would have been rejected coming
from Anthropic is rejected coming from you.

  forge agent submit --file spec.md
  forge agent submit - < spec.md
  <your-generator> | forge agent submit -`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := openBridge()
			if err != nil {
				return err
			}
			if _, ok := b.Pending(); !ok {
				return errcode.New(ErrNoPending,
					"nothing to answer in session "+b.SessionName(), nil)
			}
			content, err := readAnswer(cmd, args, fromFile)
			if err != nil {
				return err
			}
			turn, err := b.Fulfil(content)
			if err != nil {
				return errcode.New(ErrAgentFailed, "record answer", err)
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"recorded":   turn.Ordinal,
					"operation":  turn.Operation,
					"chars":      len(content),
					"session":    b.SessionName(),
					"resume_cmd": resumeCmd(b.SessionName()),
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"✓ recorded %s (%s, %d chars)\n\nnext: %s\n",
				turn.Ordinal, turn.Operation, len(content), resumeCmd(b.SessionName()))
			return nil
		},
	}
	submitCmd.Flags().StringVarP(&fromFile, "file", "f", "",
		"read the answer from this file (use '-' as the argument to read stdin instead)")

	// ── reset ─────────────────────────────────────────────────────────────────
	var confirmReset bool
	resetCmd := &cobra.Command{
		Use:   "reset",
		Short: "Discard every recorded answer for the session and start over.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			b, err := openBridge()
			if err != nil {
				return err
			}
			stats := b.Stats()
			if !confirmReset {
				return errcode.Newf(ErrAgentFailed, nil,
					"reset would discard %d recorded answer(s) in session %q; re-run with --yes to confirm",
					stats.Responses, b.SessionName())
			}
			if err := b.Reset(); err != nil {
				return errcode.New(ErrAgentFailed, "reset session", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"✓ session %s reset (%d answer(s) discarded)\n", b.SessionName(), stats.Responses)
			return nil
		},
	}
	resetCmd.Flags().BoolVarP(&confirmReset, "yes", "y", false, "confirm the reset")

	// ── sessions ──────────────────────────────────────────────────────────────
	sessionsCmd := &cobra.Command{
		Use:   "sessions",
		Short: "List agent-mode sessions in this project.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r := root
			if r == "" {
				var err error
				if r, err = os.Getwd(); err != nil {
					return errcode.New(ErrAgentFailed, "getwd", err)
				}
			}
			names, err := agentbridge.ListSessions(r)
			if err != nil {
				return errcode.New(ErrAgentFailed, "list sessions", err)
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"sessions": names})
			}
			if len(names) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(),
					"no agent-mode sessions yet — start one with: forge ship \"<feature>\" --agent-mode")
				return nil
			}
			for _, n := range names {
				fmt.Fprintln(cmd.OutOrStdout(), n)
			}
			return nil
		},
	}

	// ── loop ──────────────────────────────────────────────────────────────────
	loopCmd := &cobra.Command{
		Use:   "loop",
		Short: "Print the driver protocol to paste into an AI chat window.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprint(cmd.OutOrStdout(), DriverProtocol())
			return nil
		},
	}

	cmd.AddCommand(statusCmd, promptCmd, submitCmd, resetCmd, sessionsCmd, loopCmd)
	return cmd
}

// readAnswer resolves the answer text from --file or stdin.
//
// stdin is accepted only when explicitly requested with "-". Reading it
// implicitly would hang any invocation made from a chat tool that leaves stdin
// attached but silent, which is most of them.
func readAnswer(cmd *cobra.Command, args []string, fromFile string) (string, error) {
	useStdin := len(args) == 1 && args[0] == "-" || fromFile == "-"
	switch {
	case useStdin:
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", errcode.New(ErrAgentFailed, "read answer from stdin", err)
		}
		return string(data), nil
	case fromFile != "":
		data, err := os.ReadFile(fromFile)
		if err != nil {
			return "", errcode.New(ErrAgentFailed, "read answer file", err)
		}
		return string(data), nil
	default:
		return "", errcode.New(ErrAgentFailed,
			"no answer supplied: pass --file <path>, or '-' to read stdin", nil)
	}
}

func resumeCmd(session string) string {
	if session == "" || session == agentbridge.DefaultSession {
		return "forge ship --agent-mode"
	}
	return "forge ship --agent-mode --session " + session
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// DriverProtocol is the loop a host agent follows to run forge without an API
// key. It is exported because `forge skill install` embeds it verbatim rather
// than paraphrasing it — one wording, one source, no drift between the skill
// files on disk and the binary that enforces the protocol.
func DriverProtocol() string {
	return strings.TrimLeft(`
# Driving forge without an API key

Forge has two planes. The deterministic plane — checkpoints, quality gates,
artefact validation, scan, lint, audit and token ledgers — runs locally in Go
and needs no network. The reasoning plane — writing specs, debating
architecture, decomposing work — needs a model. Agent mode points the second
half at you instead of at a paid API.

## The loop

1. Start or continue a run:

       forge ship "<feature>" --agent-mode

   Forge runs every deterministic step it can. When it reaches a step that
   needs reasoning it prints a FORGE AGENT TURN block and exits with code 78.
   Exit 78 means "your turn" — it does not mean the change is broken.

2. Read the block. It contains the system prompt, the user prompt, and the
   output budget. Produce ONLY the artefact requested: no preamble, no
   "Here is the spec", no closing summary. Forge strips preambles and rejects
   what looks truncated, so a chatty answer costs you a retry.

3. Record the answer:

       forge agent submit --file <path>

4. Go back to step 1. Forge replays your answer at the point it was asked
   for, runs the gates against it, and either advances or hands you the next
   turn. Repeat until the run reports ready.

## Rules

- **Never report a gate as passing.** You supply text; forge supplies
  verdicts. If forge has not printed a checkpoint as green, it is not green,
  regardless of how good the artefact looks to you.
- **Never invent file paths.** If a prompt asks you to reference files, only
  reference ones you have actually read. Forge flags unverified paths in
  generated specs, and a flagged spec is a spec someone has to re-review.
- **Do not skip ahead.** Answer the turn in front of you. Forge decides what
  comes next; guessing the next three turns wastes work when a gate rejects
  the first one.
- **Lost context?** Nothing is stored in your head. Run:

       forge agent status     what run is in flight, what is owed
       forge agent prompt     re-print the pending turn in full

## Useful commands

    forge agent status                 in-flight session and pending turn
    forge agent prompt                 re-print the pending turn
    forge agent submit --file <path>   answer it
    forge agent submit -               answer it from stdin
    forge agent sessions               list concurrent sessions
    forge agent reset --yes            discard answers, start the run over

Run several features at once with --session <name> on both commands, so two
conversations never answer each other's turns.
`, "\n")
}
