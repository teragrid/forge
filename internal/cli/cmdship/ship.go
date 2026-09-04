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
// Package cmdship implements `forge ship` and its checkpoint subcommands.
//
// Subcommands (each runs a single pipeline checkpoint in isolation):
//
//	spec        – checkpoint 1: validate / generate the feature spec
//	test        – checkpoint 2: generate failing tests (TDD gate)
//	breakdown   – checkpoint 3: decompose spec into AI-friendly tasks
//	code        – checkpoint 4: generate / iterate code until tests are green
//	verify      – checkpoint 5: hygiene + scan + lint readiness check
//
// Running `forge ship` (no subcommand) runs all six checkpoints in sequence.
//
// The --dry-run mode (default in MVP) validates checkpoints without executing
// the pipeline.  Full M1 implementation arrives with each checkpoint wired to
// the LLM gateway.
package cmdship

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/agentbridge"
	"github.com/teragrid/forge/internal/audit"
	"github.com/teragrid/forge/internal/cli/cmdclean"
	"github.com/teragrid/forge/internal/cli/cmdscan"
	"github.com/teragrid/forge/internal/cli/cmdtest"
	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/gitservice"
	"github.com/teragrid/forge/internal/manifest"
	"github.com/teragrid/forge/internal/procspawn"
	"github.com/teragrid/forge/internal/telemetry"
	"github.com/teragrid/forge/internal/verbmeta"
)

// ShipEvent is a single NDJSON line emitted to stdout for each checkpoint when
// --json is active. Schema versioned at .forge/cli-schemas/ship.events.schema.json.
type ShipEvent struct {
	Event         string `json:"event"`          // spec.created | tests.generated | tasks.broken-down | task.completed | ship.passed | ship.failed
	Checkpoint    string `json:"checkpoint"`     // checkpoint name
	Status        string `json:"status"`         // ok | warning | fail | skipped
	Detail        string `json:"detail"`         // human-readable detail from the checkpoint
	TS            string `json:"ts"`             // RFC3339 timestamp
	SchemaVersion string `json:"schema_version"` // "1"
}

// checkpointEventName maps a checkpoint name to the NDJSON event type.
var checkpointEventName = map[string]string{
	"spec":      "spec.created",
	"arch":      "arch.created",
	"test":      "tests.generated",
	"breakdown": "tasks.broken-down",
	"code":      "task.completed",
	"ship":      "ship.passed",
	"verify":    "ship.passed", // deprecated alias
	"qa-verify": "qa.passed",
}

// emitEvent writes one NDJSON ShipEvent line to w.
func emitEvent(w io.Writer, cp Checkpoint, overrideEvent string) {
	evName := overrideEvent
	if evName == "" {
		evName = checkpointEventName[strings.ToLower(cp.Name)]
		if evName == "" {
			evName = strings.ToLower(cp.Name) + ".completed"
		}
	}
	ev := ShipEvent{
		Event:         evName,
		Checkpoint:    cp.Name,
		Status:        cp.Status,
		Detail:        cp.Detail,
		TS:            time.Now().UTC().Format(time.RFC3339),
		SchemaVersion: "1",
	}
	b, _ := json.Marshal(ev)
	fmt.Fprintln(w, string(b))
}

// Reserved error codes (range 3200..3299).
var (
	ErrShipFailed      = errcode.RegisterWithRemedy(errcode.Code(3200), "ship checkpoint failed", "forge ship <checkpoint> --name <slug>  # retry the failed checkpoint")
	ErrSpecAuditFailed = errcode.RegisterWithRemedy(errcode.Code(3201), "spec audit: incomplete tasks or authz gaps detected", "review .forge/specs/<slug>/spec.md and add missing ACs, then retry")
	ErrBranchFailed    = errcode.RegisterWithRemedy(errcode.Code(3203), "feature branch operation failed", "git status  # resolve merge conflicts, then forge ship --no-branch")
	ErrQAVerifyFailed  = errcode.RegisterWithRemedy(errcode.Code(3204), "QA-verify: test suite or MCP probe failed", "go test ./... -count=1  # fix failing tests, then forge ship qa-verify")
	// ErrAgentTurn is a pause, not a failure: agent mode reached a prompt the
	// host agent has not answered yet. It surfaces as exit code 78
	// (ExitAgentTurn) so a driving script can distinguish "your turn" from
	// "the change is broken", which exit 1 cannot express.
	ErrAgentTurn = errcode.RegisterWithRemedy(errcode.Code(3220),
		"agent mode: host-agent turn required",
		"forge agent prompt  # read the turn, then: forge agent submit --file <path>")
)

// agentPauseCheckpoint reports whether genErr is a paused agent-mode turn
// (IsAgentTurn) rather than a genuine LLM/provider failure, and if so fills
// in cp with a neutral "awaiting host-agent turn" status.
//
// Every checkpoint that generates an artefact via an LLMPipe must call this
// before falling back to a stub/appendFailure on error. Without it, a pause
// — the bridge has recorded a pending turn and is waiting for the host agent
// to answer — was indistinguishable from a real LLM failure: the checkpoint
// would overwrite whatever artefact was on disk with a stub template and log
// a false failure to the learned-failures file, while the bridge's own pause
// latch (Bridge.Paused) silently prevented later checkpoints in the same run
// from doing any real work either — so a single miss on, say, arch cascaded
// into every remaining checkpoint being stubbed out instead of the pipeline
// stopping to show the real prompt. See the outer caller in the ship command,
// which renders the pending turn from bridge state after RunWithOptions
// returns — that render is unaffected by what any individual checkpoint's
// Status/Detail says, but the stub writes and appendFailure calls are not,
// which is what made this worth guarding at every call site rather than
// relying on the outer check alone.
func agentPauseCheckpoint(cp *Checkpoint, operation string, genErr error) bool {
	if !IsAgentTurn(genErr) {
		return false
	}
	cp.Status = "ok"
	cp.Detail = "awaiting host-agent turn for " + operation + " — run: forge agent prompt"
	cp.AgentPaused = true
	return true
}

// ExitAgentTurn is the process exit code for a paused agent-mode run.
//
// 78 is chosen from the sysexits.h convention (EX_CONFIG, "configuration
// error") purely because it is outside the range forge already uses and is
// not a shell-reserved value (126/127/128+n). Its meaning here is forge's
// own: the pipeline is healthy and waiting on the host agent.
const ExitAgentTurn = 78

// Gate is called after each checkpoint runs (0-based idx, total count, completed cp).
// Return true to proceed to the next checkpoint; false stops the pipeline.
// A nil Gate is YOLO mode: all checkpoints run without any prompt.
type Gate func(idx, total int, cp Checkpoint) bool

// Checkpoint represents one step in the 7-checkpoint pipeline.
type Checkpoint struct {
	Name              string           `json:"name"`
	Status            string           `json:"status"` // "ok", "skipped", "warning", "fail"
	Detail            string           `json:"detail"`
	AutoAdvance       bool             `json:"auto_advance,omitempty"`       // G-009: Code sets true when all tasks done
	Approved          *bool            `json:"approved,omitempty"`           // nil=yolo/not-gated; true=approved; false=rejected
	Debate            *DebateResult    `json:"debate,omitempty"`             // populated when --yolo self-debate runs
	GapAudit          *SpecAuditResult `json:"gap_audit,omitempty"`          // TG-39: spec-vs-code audit result
	RemediationRounds int              `json:"remediation_rounds,omitempty"` // rounds of LLM-driven gap remediation
	// AgentPaused is true when this checkpoint did not run to completion but
	// instead deferred to a host-agent turn (agent mode). It is not a failure
	// and not a success — no artefact was produced, so post-checkpoint
	// side effects (quality-gate hooks, evidence policy, digests, and the
	// completion-marker file) must all be skipped: running them against a
	// nonexistent artefact either errors, reports false "unverified" noise,
	// or — for the completion marker specifically, whose path is the
	// checkpoint's own primary artefact file (e.g. arch.md) — writes bogus
	// placeholder content into the exact file the deferred turn was supposed
	// to produce, corrupting it before the host agent ever answers.
	AgentPaused bool `json:"-"`
	// Evidence records what this checkpoint's status actually rests on. A
	// status of "ok" requires at least one entry from an independent source —
	// see evidence.go. Emitted in --json so a reviewer or CI job can audit the
	// basis of a green run rather than taking the word "ok" for it.
	Evidence []Evidence `json:"evidence,omitempty"`
}

// ShipResult summarizes the ship run.
// AgentAction is a concrete next step for an LLM agent or human operator.
// AgentActions are populated in ShipResult.NextActions after a pipeline run.
type AgentAction struct {
	Priority    int    `json:"priority"`               // 1 = highest; lower number = more urgent
	Action      string `json:"action"`                 // e.g. "run: go test ./...", "edit: .forge/specs/X/spec.md"
	Reason      string `json:"reason"`                 // why this action is needed
	KBReference string `json:"kb_reference,omitempty"` // optional KB entry path that motivated this
}

type ShipResult struct {
	DryRun        bool         `json:"dry_run"`
	Yolo          bool         `json:"yolo,omitempty"`
	Interactive   bool         `json:"interactive,omitempty"`
	DebateEnabled bool         `json:"debate_enabled,omitempty"`
	Checkpoints   []Checkpoint `json:"checkpoints"`
	Ready         bool         `json:"ready"`
	Message       string       `json:"message"`
	FeatureBranch string       `json:"feature_branch,omitempty"` // branch used for this ship run

	// Agent/LLM-friendly fields — populated after every successful pipeline run.

	// AgentHints carries per-checkpoint hints for LLM agents consuming this result.
	// Key = lower-cased checkpoint name; value = list of actionable hint strings.
	AgentHints map[string][]string `json:"agent_hints,omitempty"`

	// NextActions is an ordered list of concrete steps the next agent/human should take.
	// Populated from checkpoint details, gap audit findings, and KB-derived guidance.
	NextActions []AgentAction `json:"next_actions,omitempty"`

	// TokenUsage tracks the approximate tokens consumed per checkpoint (LLM calls only).
	// Key = lower-cased checkpoint name; value = estimated token count.
	TokenUsage map[string]int `json:"token_usage,omitempty"`

	// KBEntriesUsed lists the KB entry IDs injected via InvokeWithKnowledge across the run.
	// Enables audit trails and downstream KB usage analysis.
	KBEntriesUsed []string `json:"kb_entries_used,omitempty"`

	// Complexity is the tier assigned to the feature description by the classifier.
	// Drives adaptive token budgets (P1) and checkpoint selection (nano skips some phases).
	Complexity ComplexityTier `json:"complexity,omitempty"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb: "ship",
		Summary: "Deploy a change through the 7-checkpoint pipeline " +
			"(spec → arch → test → breakdown → code → ship → qa-verify). " +
			"Each checkpoint requires reviewer approval before the next runs (interactive mode). " +
			"Use --yolo to skip all approval gates. " +
			"Run a single checkpoint with: forge ship spec|arch|test|breakdown|code|ship|qa-verify. " +
			"M1 full impl; MVP is --dry-run preview.",
		Inputs: []string{
			"[subcommand]: spec | arch | test | breakdown | code | ship | qa-verify (optional; runs all when omitted)",
			"--skip-checkpoint qa-verify (skip QA agent; useful when no test runner is configured)",
			"--dry-run (validate checkpoints without executing; default in MVP)",
			"--description <msg> (what this change does; required for full pipeline in M1)",
			"--yolo (skip all approval gates — activates 6-role self-debate for quality polishing)",
			"--json (machine-readable output; also disables interactive prompts)",
			"--no-branch (skip automatic feature-branch creation; work on the current branch)",
		},
		Outputs: []string{"stdout: checkpoint status (text or JSON)"},
		SideEffects: []string{
			"--dry-run has no side effects; full workflow (M1) will commit, tag, and deploy",
			"creates and checks out feature/<slug> branch when running the full pipeline from a protected branch (main/master/develop/dev/trunk)",
		},
		GatesTouched: []string{"§16.5.2 ship workflow", "§4 7-checkpoint pipeline"},
		ErrorCodes:   []errcode.Code{ErrShipFailed, ErrBranchFailed, ErrQAVerifyFailed},
	})
}

// makeInteractiveGate returns a Gate that reads y/N from scanner and writes
// prompts to out. It is called between checkpoints (never after the last one).
func makeInteractiveGate(scanner *bufio.Scanner, out io.Writer) Gate {
	return func(idx, total int, cp Checkpoint) bool {
		marker := "○"
		switch cp.Status {
		case "ok":
			marker = "✓"
		case "fail":
			marker = "✗"
		case "warning":
			marker = "\u25b3" // △ WHITE UP-POINTING TRIANGLE (Consolas-safe)
		}
		fmt.Fprintf(out, "\n  [%d/%d] %s %s — %s\n", idx+1, total, marker, cp.Name, cp.Detail)
		fmt.Fprintf(out, "       → Approve and continue to checkpoint %d/%d? [y/N] ", idx+2, total)
		if !scanner.Scan() {
			return true // EOF / closed stdin → auto-approve (non-interactive caller)
		}
		return strings.ToLower(strings.TrimSpace(scanner.Text())) == "y"
	}
}

// New returns the cobra command with checkpoint subcommands.
func New() *cobra.Command {
	var (
		dryRun         bool
		description    string
		specName       string // --name/-n: override spec directory name for 'forge ship spec'
		asJSON         bool
		humanMode      bool // --human: force human-readable output, opt-out of LLM mode auto-detection
		yolo           bool
		quick          bool   // --quick: lightweight spec+code only (skip test+breakdown+verify)
		yes            bool   // --yes: auto-approve all gates (alias for --yolo for non-YOLO users)
		from           string // --from: resume from a named checkpoint
		skipCheckpoint string // --skip-checkpoint: skip a named checkpoint
		pr             bool   // --pr: create a draft GitHub PR after all checkpoints pass
		resume         bool   // --resume: resume from first incomplete checkpoint (G-002)
		rootDir        string // --root: project root (default: cwd); primarily for testing
		noBranch       bool   // --no-branch: skip automatic feature-branch creation
		strictTesting  bool   // --strict-testing: [no-op since 1.8.2] force the qa-verify gate on
		noStrictTest   bool   // --no-strict-testing: opt out of the 4-stage testing-pipeline.md gate
		agentMode      bool   // --agent-mode: reasoning plane = host agent, no API key
		agentSession   string // --session: which agent-mode conversation this run belongs to
		strictReplay   bool   // --strict-replay: never replay an answer by position when the prompt changed
	)

	// bindFlags attaches shared flags to a subcommand or the parent.
	bindFlags := func(c *cobra.Command) {
		c.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "preview what would happen without making LLM calls or git operations")
		c.Flags().StringVar(&description, "description", "", "what this change does (deprecated: use positional arg instead)")
		c.Flags().BoolVarP(&asJSON, "json", "j", false, "emit machine-readable JSON")
		c.Flags().BoolVar(&humanMode, "human", false, "force human-readable output (opt-out of LLM mode auto-detection)")
		c.Flags().BoolVarP(&yolo, "yolo", "Y", false, "skip all approval gates (ship without review prompts)")
		c.Flags().BoolVarP(&quick, "quick", "Q", false, "lightweight run: spec+code only (skips test, breakdown, verify)")
		c.Flags().BoolVarP(&yes, "yes", "y", false, "auto-approve all checkpoint gates (alias for --yolo)")
		c.Flags().StringVarP(&from, "from", "f", "", "resume pipeline from this checkpoint (e.g. --from=code)")
		c.Flags().StringVarP(&skipCheckpoint, "skip-checkpoint", "s", "", "skip a specific checkpoint by name")
		c.Flags().BoolVarP(&pr, "pr", "p", false, "create a draft GitHub PR after all checkpoints pass (requires gh CLI)")
		c.Flags().StringVarP(&rootDir, "root", "r", "", "project root (default: cwd)")
		c.Flags().BoolVarP(&resume, "resume", "R", false, "resume from first incomplete checkpoint (replaces: forge ship resume <feature>)")
		c.Flags().BoolVarP(&noBranch, "no-branch", "B", false, "skip automatic feature-branch creation; work on the current branch")
		c.Flags().BoolVar(&strictTesting, "strict-testing", false,
			"[no-op since 1.8.2 — the gate is on by default] force the 4-stage testing gate on, overriding \"strict-testing: false\" in .forge/hooks.yaml")
		c.Flags().BoolVar(&noStrictTest, "no-strict-testing", false,
			"ship without 4-stage testing evidence: downgrades the qa-verify testing-pipeline.md gate from blocking back to advisory for this run")
		c.Flags().StringVarP(&specName, "name", "n", "",
			"name of the spec directory in .forge/specs/ (overrides slug derived from description; applies to all checkpoints)")
		c.Flags().BoolVar(&agentMode, "agent-mode", false,
			"drive the pipeline from your own AI chat instead of an API key: forge pauses at each reasoning step, hands you the prompt, and resumes once you submit the answer")
		c.Flags().StringVar(&agentSession, "session", agentbridge.DefaultSession,
			"agent-mode session name — keeps concurrent features from answering each other's turns")
		c.Flags().BoolVar(&strictReplay, "strict-replay", false,
			"agent-mode: re-ask a prompt whose text changed instead of replaying the answer recorded at the same position")
	}

	// runCheckpoint is the shared body: run only the named checkpoint(s).
	runCheckpoint := func(cmd *cobra.Command, names []string) error {
		root := rootDir
		if root == "" {
			var err error
			root, err = os.Getwd()
			if err != nil {
				return errcode.New(ErrShipFailed, "getwd", err)
			}
		}

		// --yes is an alias for --yolo (more approachable for non-YOLO users).
		if yes {
			yolo = true
		}

		// LLM mode: suppress interactive y/N gates so an LLM driving forge is
		// never blocked (AC-3). TTY detection is intentionally excluded here so
		// that `go test` (non-TTY but human developer) never silently skips
		// gates.
		//
		// NO_COLOR=1 used to be accepted here too, and must not be. NO_COLOR is
		// defined (no-color.org) as a purely *presentational* signal: it asks
		// software not to emit ANSI colour. People set it for accessibility, for
		// terminals that render escape codes badly, or just preference — and
		// they set it globally, in a shell profile, once, years ago.
		//
		// Reading it as "an LLM is driving me" meant every one of those users
		// silently lost every approval gate in the pipeline. forge ship would
		// auto-approve all six checkpoints and never ask, and nothing in the
		// output said so. A signal about how text is *displayed* was deciding
		// whether a human reviews the change. Colour is still suppressed for
		// NO_COLOR — see internal/cli/banner — which is all it ever asked for.
		if !yolo && !asJSON && !humanMode && os.Getenv("FORGE_LLM_MODE") == "1" {
			yolo = true
			// Say it out loud. Skipping review is the kind of thing that must
			// never happen quietly, even when it is the correct behaviour.
			fmt.Fprintln(cmd.ErrOrStderr(),
				"note: FORGE_LLM_MODE=1 — approval gates auto-approved (unset it, or pass --human, to review each checkpoint)")
		}

		// --quick: run spec+code only (skip test, breakdown, verify).
		if quick && len(names) == 0 {
			names = []string{"spec", "code"}
		}

		// --from: drop all checkpoints before the named one.
		if from != "" && len(names) == 0 {
			order := []string{"spec", "arch", "test", "breakdown", "code", "ship", "qa-verify"}
			found := false
			for _, cp := range order {
				if cp == from {
					found = true
				}
				if found {
					names = append(names, cp)
				}
			}
			if !found {
				return errcode.Newf(ErrShipFailed, nil,
					"--from: unknown checkpoint %q; one of: spec, arch, test, breakdown, code, ship, qa-verify", from)
			}
		}

		// --skip-checkpoint: remove a named checkpoint from the run list.
		if skipCheckpoint != "" {
			if len(names) == 0 {
				names = []string{"spec", "arch", "test", "breakdown", "code", "ship"}
			}
			filtered := names[:0]
			for _, n := range names {
				if n != skipCheckpoint {
					filtered = append(filtered, n)
				}
			}
			names = filtered
			// G-014: Audit skip-checkpoint usage.
			if al, aErr := audit.Open(audit.DefaultPath); aErr == nil {
				_, _ = al.Append(audit.Entry{
					Verb:   "ship",
					Action: "skip_checkpoint",
					Detail: map[string]string{"checkpoint": skipCheckpoint},
				})
			}
		}

		// Feature branch: auto-create feature/<slug> when on a protected branch.
		// Only for full-pipeline runs (not single checkpoints) with a description.
		var branchRes featureBranchResult
		if !noBranch && len(names) == 0 && description != "" {
			branchRes = ensureFeatureBranch(root, slugify(description))
			if !asJSON {
				// In JSON mode, branch info is in the result; don't pollute stdout.
				if branchRes.Warning != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "△ branch: %s\n", branchRes.Warning)
				} else if branchRes.Created {
					fmt.Fprintf(cmd.OutOrStdout(), "✓ created branch %s — pipeline artefacts will land here\n", branchRes.Branch)
				}
			}
		}

		// Build approval gate: only for the full pipeline (nil names = all checkpoints).
		// Single-checkpoint subcommands never need approval.
		// --yolo and --json both disable interactive prompts.
		var gate Gate
		if !yolo && !asJSON && len(names) == 0 {
			gate = makeInteractiveGate(bufio.NewScanner(cmd.InOrStdin()), cmd.OutOrStdout())
		}
		// Self-debate: active when --yolo on the full pipeline.
		// Single-checkpoint subcommands don't debate (no spec context available).
		runOpts := RunOptions{
			Root:            root,
			Description:     description,
			SpecName:        specName,
			Names:           names,
			Gate:            gate,
			CreatePR:        pr,
			DryRun:          dryRun,
			StrictTesting:   strictTesting,
			NoStrictTesting: noStrictTest,
		}
		if yolo && len(names) == 0 {
			runOpts.DebateOpts = &DebateOptions{
				Feature:   description,
				MaxRounds: 3,
				DryRun:    dryRun,
			}
		}

		// Auto-fallback: when the pipeline is not already in agent mode and the
		// configured LLM provider is unusable for a permanent reason (no
		// provider configured, auth failed, or a hard invalid_request such as
		// "credit balance too low" — see llmpipe.go probeProviderUsable), drive
		// the run via the host agent instead of hard-failing every LLM
		// checkpoint. This is the documented escape hatch (--agent-mode) turned
		// on automatically instead of requiring the operator to notice the
		// failure and re-invoke with the flag. FORGE_NO_AGENT_FALLBACK=1 opts
		// out for callers that want a hard failure instead (e.g. CI jobs that
		// should not silently pause on a human/host-agent turn).
		if !agentMode && !dryRun && os.Getenv("FORGE_AGENT_MODE") != "1" && os.Getenv("FORGE_NO_AGENT_FALLBACK") != "1" {
			if usable, reason := probeProviderUsable(); !usable {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"note: configured LLM provider is unusable (%s) — falling back to --agent-mode "+
						"automatically (set FORGE_NO_AGENT_FALLBACK=1 to disable)\n", reason)
				agentMode = true
			}
		}

		// Agent mode: swap the reasoning plane from a paid provider to the
		// host agent. The deterministic plane is untouched — same checkpoints,
		// same gates, same artefact validation.
		var bridge *agentbridge.Bridge
		if agentMode || os.Getenv("FORGE_AGENT_MODE") == "1" {
			var bErr error
			bridge, bErr = agentbridge.Open(root, agentSession)
			if bErr != nil {
				return errcode.New(ErrAgentTurn, "open agent bridge", bErr)
			}
			bridge.StrictReplay = strictReplay
			// ISSUE 4 fix: a bare continuation (no --name, no description — the
			// shape of the hint forge itself prints after a submit: "next: forge
			// ship --agent-mode") must resume the same feature this session was
			// already driving, not fall through to whatever spec/checkpoint
			// resolution does with an empty name and description. Resolve from
			// the session's own recorded identity before SetFeature runs, and
			// propagate into runOpts so RunWithOptions targets the right spec.
			if description == "" && specName == "" {
				if priorFeature, priorSlug := bridge.Feature(); priorSlug != "" || priorFeature != "" {
					description, specName = priorFeature, priorSlug
					runOpts.Description, runOpts.SpecName = description, specName
				}
			}
			if bridge.SetFeature(description, specName) {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"note: session %q was driving a different feature — recorded answers reset for %q "+
						"(use --session <name> to run multiple features concurrently without this)\n",
					agentSession, description)
			}
			runOpts.AgentBridge = bridge
			// Interactive y/N gates would block a chat-driven run between
			// turns, and the host agent has no stdin to answer them with.
			runOpts.Gate = nil
			yolo = true
		}
		// G-004: --yes && --json → NDJSON event stream. --json alone → single JSON object (backward compat).
		if yolo && asJSON {
			runOpts.EventWriter = cmd.OutOrStdout()
		}
		res := RunWithOptions(runOpts)

		// A paused run is not a failed run. The pause is checked before the
		// result is rendered so the host agent gets the turn block instead of
		// a wall of checkpoint output ending in a misleading "failed" — the
		// checkpoint did not fail, it never got its answer.
		if turn, paused := bridge.Pending(); paused && bridge.Paused() {
			return renderAgentTurn(cmd, turn, agentSession, asJSON)
		}

		res.Yolo = yolo
		res.Interactive = gate != nil
		res.DebateEnabled = runOpts.DebateOpts != nil
		if branchRes.Branch != "" && branchRes.Warning == "" {
			// Attach the feature branch to the result for JSON output and renderText MR hint.
			res.FeatureBranch = branchRes.Branch
		}
		if asJSON && !yolo {
			// Backward-compat: single JSON object when --json without --yes.
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if encErr := enc.Encode(res); encErr != nil {
				return encErr
			}
		} else if !asJSON {
			renderText(cmd, res)
		}
		// When yolo+json: events already written via EventWriter.
		if !res.Ready {
			return errcode.New(ErrShipFailed, "one or more checkpoints failed or were rejected; see output above", nil)
		}
		return nil
	}

	cmd := &cobra.Command{
		Use:   "ship [<feature>] [spec|arch|test|breakdown|code|ship|qa-verify] [flags]",
		Short: "Deploy a change through the 7-checkpoint pipeline.",
		Long: "forge ship walks a change through 7 checkpoints:\n" +
			"  (1) spec      — validate or generate the feature spec\n" +
			"  (2) arch      — multi-role architecture debate → ADR document\n" +
			"  (3) test      — generate failing tests (TDD gate)\n" +
			"  (4) breakdown — decompose spec into AI-friendly tasks\n" +
			"  (5) code      — generate / iterate code until tests pass\n" +
			"  (6) ship      — hygiene + scan + lint readiness check\n" +
			"  (7) qa-verify — QA agent: probe MCP server tools or run native test suite\n\n" +
			"Run a single checkpoint with: forge ship spec|arch|test|breakdown|code|ship|qa-verify\n" +
			"Run all seven checkpoints with: forge ship (no subcommand)\n\n" +
			"Use --dry-run to preview checkpoints without making LLM calls or git operations.\n\n" +
			"Examples:\n" +
			"  forge ship \"add rate limiting\"                — full 7-checkpoint pipeline\n" +
			"  forge ship spec \"add rate limiting\"           — spec checkpoint only\n" +
			"  forge ship qa-verify \"add rate limiting\"      — QA agent only\n" +
			"  forge ship \"add rate limiting\" --dry-run      — preview without writing\n" +
			"  forge ship \"add rate limiting\" --resume       — continue from last checkpoint",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Intercept built-in cobra subcommand words that can reach RunE
			// when a positional arg is also accepted (cobra v1.8 behaviour).
			if len(args) == 1 && args[0] == "help" {
				return cmd.Help()
			}
			// G-001: positional <feature> arg.
			if len(args) == 1 {
				if description != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "note: --description ignored; using positional feature %q\n", args[0])
				}
				description = args[0]
			} else if cmd.Flags().Changed("description") {
				// G-001: --description is a deprecated alias.
				fmt.Fprintf(cmd.OutOrStdout(), "deprecation: --description is deprecated; use positional arg instead: forge ship %q\n", slugify(description))
			}
			// G-002: --resume flag.
			if resume {
				return runResumeFlag(cmd, description, rootDir)
			}
			return runCheckpoint(cmd, nil) // nil = all checkpoints
		},
	}
	bindFlags(cmd)

	// â”€â”€ Checkpoint subcommands â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

	makeCheckpointCmd := func(name, short string) *cobra.Command {
		c := &cobra.Command{
			Use:   name + ` [<description>]`,
			Short: short,
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) == 1 {
					if description != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "note: --description ignored; using positional arg %q\n", args[0])
					}
					description = args[0]
				} else if cmd.Flags().Changed("description") {
					fmt.Fprintf(cmd.OutOrStdout(), "tip: drop the flag — run: forge ship %s %q\n", name, description)
				}
				return runCheckpoint(cmd, []string{name})
			},
		}
		bindFlags(c)
		return c
	}

	// G-003: checkpoint 6 is "ship"; "verify" kept as deprecated alias.
	verifyDeprecated := makeCheckpointCmd("verify",
		"[deprecated] Checkpoint 6: use 'forge ship ship' instead.")
	verifyDeprecated.Deprecated = "use 'forge ship ship' instead"
	specSubCmd := makeCheckpointCmd("spec",
		"Checkpoint 1: validate or generate the feature spec")
	cmd.AddCommand(
		specSubCmd,
		makeCheckpointCmd("arch",
			"Checkpoint 2: multi-role architecture debate → ADR document"),
		makeCheckpointCmd("test",
			"Checkpoint 3: generate failing tests before any code (TDD gate)"),
		makeCheckpointCmd("breakdown",
			"Checkpoint 4: decompose the spec into AI-friendly task list"),
		makeCheckpointCmd("code",
			"Checkpoint 5: generate / iterate code until tests pass"),
		makeCheckpointCmd("ship",
			"Checkpoint 6: hygiene + scan + lint ship-readiness check"),
		makeCheckpointCmd("qa-verify",
			"Checkpoint 7: QA agent — probe MCP server tools or run native test suite"),
		verifyDeprecated,
	)

	// â”€â”€ Status + resume subcommands (spec §4 ship sub-verbs) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

	statusCmd := newStatusCmd()

	// G-002: kept as deprecated alias for --resume flag.
	resumeCmd := &cobra.Command{
		Use:        "resume <feature>",
		Deprecated: "use 'forge ship <feature> --resume' instead",
		Short:      "Resume a paused pipeline from the last completed checkpoint.",
		Long: "Reads .forge/specs/<feature>/ to determine which checkpoints are complete,\n" +
			"then continues from the next pending checkpoint.\n\n" +
			"Equivalent to: forge ship --from=<next-checkpoint> <feature>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			feature := args[0]
			root := rootDir
			if root == "" {
				var err error
				root, err = os.Getwd()
				if err != nil {
					return errcode.New(ErrShipFailed, "getwd", err)
				}
			}
			specsDir := filepath.Join(root, ".forge", "specs", feature)
			if _, err := os.Stat(specsDir); os.IsNotExist(err) {
				return errcode.Newf(ErrShipFailed, nil,
					"feature %q not found in .forge/specs/; start with: forge ship %s", feature, feature)
			}
			// Find first pending checkpoint
			checkpoints := []string{"spec", "arch", "test", "breakdown", "code", "ship", "qa-verify"}
			resumeFrom := ""
			for _, cp := range checkpoints {
				cpFile := filepath.Join(specsDir, cp+".md")
				if _, err := os.Stat(cpFile); os.IsNotExist(err) {
					resumeFrom = cp
					break
				}
			}
			if resumeFrom == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "forge ship resume: %s — all checkpoints complete\n", feature)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "forge ship resume: %s — resuming from checkpoint %q\n", feature, resumeFrom)
			description = feature
			from = resumeFrom
			return runCheckpoint(cmd, nil)
		},
	}
	bindFlags(resumeCmd)

	cmd.AddCommand(statusCmd, resumeCmd)

	return cmd
}

// runResumeFlag implements G-002: forge ship <feature> --resume.
// It reads .forge/specs/<slug>/ to find the first pending checkpoint and
// continues from there, printing a deprecation hint for old resume subcommand callers.
// boolFlag reads a bool flag off cmd, returning false when it is absent.
// Tolerating absence keeps callers that build a bare command (tests, and the
// programmatic API) from having to register every flag the CLI happens to own.
func boolFlag(cmd *cobra.Command, name string) bool {
	f := cmd.Flags().Lookup(name)
	return f != nil && f.Value.String() == "true"
}

func runResumeFlag(cmd *cobra.Command, feature, rootDir string) error {
	root := rootDir
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return errcode.New(ErrShipFailed, "getwd", err)
		}
	}
	slug := slugify(feature)
	specsDir := filepath.Join(root, ".forge", "specs", slug)
	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		return errcode.Newf(ErrShipFailed, nil,
			"feature %q not found in .forge/specs/; start with: forge ship %s", feature, slug)
	}
	checkpoints := []string{"spec", "arch", "test", "breakdown", "code", "ship", "qa-verify"}
	resumeFrom := ""
	for _, cp := range checkpoints {
		cpFile := filepath.Join(specsDir, cp+".md")
		if _, err := os.Stat(cpFile); os.IsNotExist(err) {
			resumeFrom = cp
			break
		}
	}
	if resumeFrom == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "forge ship resume: %s \u2014 all checkpoints complete\n", slug)
		return nil
	}
	// Determine JSON mode before printing text.
	jsonFlag := cmd.Flags().Lookup("json")
	isJSON := jsonFlag != nil && jsonFlag.Value.String() == "true"
	if !isJSON {
		fmt.Fprintf(cmd.OutOrStdout(), "forge ship --resume: %s \u2014 resuming from checkpoint %q\n", slug, resumeFrom)
	}
	// Build names list from resumeFrom onward.
	var names []string
	found := false
	for _, cp := range checkpoints {
		if cp == resumeFrom {
			found = true
		}
		if found {
			names = append(names, cp)
		}
	}
	_ = found
	// --resume must honour the run flags it was given. RunCheckpoints takes no
	// options, so routing through it silently discarded every flag on the
	// command line: `forge ship <f> --resume --no-strict-testing` re-enabled
	// the gate the user had just waived, and `--resume --agent-mode` would
	// have dialled a provider. Read the flags off the command instead — the
	// resumed run is the same pipeline and must obey the same switches.
	res := RunWithOptions(RunOptions{
		Root:            root,
		Description:     feature,
		Names:           names,
		StrictTesting:   boolFlag(cmd, "strict-testing"),
		NoStrictTesting: boolFlag(cmd, "no-strict-testing"),
	})
	// Respect --json flag (single JSON object when --json without --yes).
	if isJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	if res.Ready {
		fmt.Fprintf(cmd.OutOrStdout(), "forge ship --resume: %s \u2014 pipeline complete\n", slug)
	} else {
		return errcode.New(ErrShipFailed, "pipeline not complete after resume", nil)
	}
	return nil
}

// Run executes the full 6-checkpoint dry-run validation (backward-compat entry point).
// DryRun is always true for this entry point to preserve backward-compatible behaviour.
func Run(root, description string) *ShipResult {
	return RunWithOptions(RunOptions{Root: root, Description: description, DryRun: true})
}

// â”€â”€ Per-checkpoint evaluators â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// specFamilyList returns a compact comma-separated list of test families.
func specFamilyList(families []cmdtest.Family) string {
	if len(families) == 0 {
		return "(none)"
	}
	parts := make([]string, len(families))
	for i, f := range families {
		parts[i] = string(f)
	}
	return strings.Join(parts, ", ")
}

// specYAMLContext builds a compact text summary of a TestSpec for LLM context.
func specYAMLContext(spec *cmdtest.TestSpec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "feature: %s\n", spec.Feature)
	if spec.Description != "" {
		fmt.Fprintf(&b, "description: %s\n", spec.Description)
	}
	fmt.Fprintf(&b, "families: %s\n", specFamilyList(spec.Families))
	fmt.Fprintf(&b, "cases (%d total):\n", len(spec.Cases))
	for _, c := range spec.Cases {
		fmt.Fprintf(&b, "  [%s] %s (%s)\n", c.ID, c.Name, c.Type)
		if c.Assert != "" {
			fmt.Fprintf(&b, "    assert: %s\n", c.Assert)
		}
	}
	return b.String()
}

// checkSpec validates or generates the feature spec.
// With an LLMPipe: if the spec already exists it is reviewed and enhanced;
// if it does not exist it is generated from the description.
// Without an LLMPipe (no provider configured): a Markdown stub is written.
// When a pre-generated spec.yml (from `forge test spec`) exists it is loaded
// to enrich the LLM call via InvokeWithKnowledge and surfaced in the detail.
func checkSpec(root, description, specName string, pipe *LLMPipe) Checkpoint {
	cp := Checkpoint{Name: "Spec"}
	// G-011: surface recent spec failures as context for the LLM.
	recentSpecFailures := loadRecentFailures(root, "spec", 3)
	specsDir := filepath.Join(root, ".forge", "specs")
	if description != "" || specName != "" {
		// Determine the spec directory name: --name/-n flag takes priority over the
		// slug derived from the description, letting users target a known spec
		// directory directly (e.g. `forge ship spec --name login`).
		slug := specName
		if slug == "" {
			slug = slugify(description)
		}
		// When only --name is given (no description), use it as LLM feature context.
		if description == "" {
			description = specName
		}
		specFile := filepath.Join(specsDir, slug, "spec.md")
		yamlSpecPath := filepath.Join(specsDir, slug, "spec.yml")

		// G-009 (workspace-context phase): collect deterministic project context
		// before any LLM call so the spec reflects the actual tech stack,
		// conventions, recent changes, and existing features.
		wsCtx := collectWorkspaceContext(root, slug)
		wsSection := ""
		if wsCtx.Content != "" {
			wsSection = "\n\n## Workspace Context\n" + wsCtx.Content
		}

		// Load a pre-generated YAML spec (from `forge test spec`) if present.
		// Errors are silently ignored; a nil ySpec means fall back to spec.md-only logic.
		ySpec, _ := cmdtest.ReadSpec(yamlSpecPath)

		if _, err := os.Stat(specFile); err == nil {
			// spec.md exists — attempt LLM review, enriched with YAML context when available.
			if pipe != nil {
				existing, _ := os.ReadFile(specFile)
				specReviewSystem := "You are a senior software architect reviewing a feature specification. " +
					"Improve its acceptance criteria (Given/When/Then format), NFRs, and edge cases. " +
					"Return the full improved specification in Markdown."
				if recentSpecFailures != "" {
					specReviewSystem = recentSpecFailures + "\n" + specReviewSystem
				}
				userPrompt := fmt.Sprintf("Feature: %s%s\n\nCurrent spec:\n%s", description, wsSection, string(existing))
				if ySpec != nil {
					userPrompt = fmt.Sprintf(
						"Feature: %s%s\n\nYAML test spec (%d cases, families: %s):\n%s\n\nCurrent spec.md:\n%s",
						description, wsSection, len(ySpec.Cases), specFamilyList(ySpec.Families),
						specYAMLContext(ySpec), string(existing),
					)
				}
				reviewFn := func() (string, bool, error) {
					// 8000: 4096 reliably truncated on a full spec (What/Why/AC
					// in Given-When-Then/NFRs/Out-of-scope) for anything but a
					// trivial feature — arch.go's own generation budget was
					// bumped 3000->6000 for the same reason (root-caused
					// 2026-07-19) but this call site was left at 4096 by
					// mistake. AnthropicAdapter.continueOnMaxTokens (see
					// anthropic.go) softens this by continuing a cut-off
					// response instead of discarding it, but it replays at the
					// *same* budget — it does not raise it — so an
					// undersized base budget still yields "truncated after
					// retry" for verbose specs. Fix the budget here, don't
					// rely on continuation to cover for it.
					if ySpec != nil {
						// KB-enriched review when a YAML spec is present.
						return pipe.InvokeWithKnowledgeChecked(
							"ship:spec:review", "",
							specReviewSystem, userPrompt, 8000,
							"spec", "unit", "spec", []string{"test-design", "quality-gate"},
						)
					}
					return pipe.InvokeChecked(
						"ship:spec:review", "",
						specReviewSystem, userPrompt, 8000,
					)
				}
				// J8/J9: strip conversational preamble and detect truncation
				// before overwriting an existing, presumably-good spec.md.
				reviewed, complete, reviewErr := generateWithValidation(reviewFn)
				if agentPauseCheckpoint(&cp, "ship:spec:review", reviewErr) {
					return cp
				}
				if reviewErr != nil {
					cp.Status = "ok"
					if ySpec != nil {
						cp.Detail = fmt.Sprintf(
							"spec found (spec.yml: %d cases, families: %s) [LLM:%s — %s]",
							len(ySpec.Cases), specFamilyList(ySpec.Families),
							pipe.ProviderName(), llmErrNote(reviewErr))
					} else {
						cp.Detail = fmt.Sprintf("spec found: .forge/specs/%s/spec.md [LLM:%s — %s]",
							slug, pipe.ProviderName(), llmErrNote(reviewErr))
					}
					appendFailure(root, "spec", description, cp.Detail+" | full: "+llmErrFull(reviewErr))
					return cp
				}
				if reviewed != "" && !complete {
					// J9: never overwrite a working spec.md with a truncated
					// review — keep the existing file and log the failure.
					cp.Status = "warning"
					cp.Detail = fmt.Sprintf(
						"spec review truncated/incomplete after retry (LLM:%s) — kept existing spec.md: "+
							".forge/specs/%s/spec.md unchanged", pipe.ProviderName(), slug)
					appendFailure(root, "spec", description, cp.Detail)
					return cp
				}
				if reviewed != "" {
					reviewed = appendUnverifiedPathsWarning(root, reviewed)
					_ = os.WriteFile(specFile, []byte(reviewed), 0o600)
					cp.Status = "ok"
					if ySpec != nil {
						cp.Detail = fmt.Sprintf(
							"spec reviewed via KB (%s) — spec.yml: %d cases, families: %s",
							pipe.ProviderName(), len(ySpec.Cases), specFamilyList(ySpec.Families))
					} else {
						cp.Detail = fmt.Sprintf("spec reviewed and enhanced by %s: .forge/specs/%s/spec.md",
							pipe.ProviderName(), slug)
					}
					return cp
				}
			}
			cp.Status = "ok"
			if ySpec != nil {
				cp.Detail = fmt.Sprintf("spec found (spec.yml: %d cases, families: %s): .forge/specs/%s/spec.md",
					len(ySpec.Cases), specFamilyList(ySpec.Families), slug)
			} else {
				cp.Detail = fmt.Sprintf("spec found: .forge/specs/%s/spec.md", slug)
			}
			return cp
		}

		// spec.md does not exist.
		// If a YAML spec is present, generate spec.md from it (KB-enriched when LLM available).
		if ySpec != nil {
			if err := os.MkdirAll(filepath.Join(specsDir, slug), 0o755); err == nil {
				specContent := ""
				if pipe != nil {
					specGenSystem := "You are a senior software architect. " +
						"Generate a complete feature specification in Markdown from the provided YAML test spec. " +
						"Include: What, Why, Acceptance Criteria (Given/When/Then format derived from the test cases), " +
						"Non-functional requirements, and Out of scope."
					if recentSpecFailures != "" {
						specGenSystem = recentSpecFailures + "\n" + specGenSystem
					}
					genFn := func() (string, bool, error) {
						// 8000 — see the ship:spec:review budget comment above;
						// same undersized-budget root cause applies here.
						return pipe.InvokeWithKnowledgeChecked(
							"ship:spec:generate-from-yaml", "",
							specGenSystem,
							fmt.Sprintf("Generate spec.md for feature: %s%s\n\n%s", description, wsSection, specYAMLContext(ySpec)),
							8000,
							"spec", "unit", "spec", []string{"test-design", "quality-gate"},
						)
					}
					// J8/J9: strip preamble and detect truncation before writing.
					generated, genComplete, genErr := generateWithValidation(genFn)
					if agentPauseCheckpoint(&cp, "ship:spec:generate-from-yaml", genErr) {
						return cp
					}
					switch {
					case genErr != nil:
						specContent = specStub(description)
						cp.Detail = fmt.Sprintf(
							"spec.md stub created from spec.yml (%d cases) [LLM:%s — %s]: .forge/specs/%s/spec.md — edit before continuing",
							len(ySpec.Cases), pipe.ProviderName(), llmErrNote(genErr), slug)
						appendFailure(root, "spec", description, cp.Detail+" | full: "+llmErrFull(genErr))
					case generated != "" && !genComplete:
						// J9: never write a truncated/preamble-broken spec.md as if it succeeded.
						specContent = specStub(description)
						cp.Detail = fmt.Sprintf(
							"spec.md generation truncated/incomplete after retry (LLM:%s) — stub created from spec.yml "+
								"(%d cases): .forge/specs/%s/spec.md — edit before continuing",
							pipe.ProviderName(), len(ySpec.Cases), slug)
						appendFailure(root, "spec", description, cp.Detail)
					case generated != "":
						specContent = appendUnverifiedPathsWarning(root, generated)
						cp.Detail = fmt.Sprintf("spec.md generated from spec.yml (%d cases) by %s: .forge/specs/%s/spec.md",
							len(ySpec.Cases), pipe.ProviderName(), slug)
					default:
						specContent = specStub(description)
						cp.Detail = fmt.Sprintf("spec.md stub created from spec.yml (%d cases): .forge/specs/%s/spec.md",
							len(ySpec.Cases), slug)
					}
				} else {
					specContent = specStub(description)
					cp.Detail = fmt.Sprintf(
						"spec.yml found (%d cases, families: %s) — spec.md stub written: .forge/specs/%s/spec.md"+
							" (run 'forge config set llm.provider <name>' or set ANTHROPIC_API_KEY / OPENAI_API_KEY)",
						len(ySpec.Cases), specFamilyList(ySpec.Families), slug)
				}
				if err := os.WriteFile(specFile, []byte(specContent), 0o600); err == nil {
					cp.Status = "ok"
					return cp
				}
			}
			cp.Status = "ok"
			cp.Detail = fmt.Sprintf("spec.yml found (%d cases) — .forge/ not writable", len(ySpec.Cases))
			return cp
		}

		// Neither spec.yml nor spec.md — generate via LLM or write a stub.
		if err := os.MkdirAll(filepath.Join(specsDir, slug), 0o755); err == nil {
			specContent := ""
			if pipe != nil {
				specGenSystem := "You are a senior software architect. Generate a feature specification in Markdown. " +
					"Include: What, Why, Acceptance Criteria (Given/When/Then format), " +
					"Non-functional requirements, and Out of scope."
				if recentSpecFailures != "" {
					specGenSystem = recentSpecFailures + "\n" + specGenSystem
				}
				genFn := func() (string, bool, error) {
					// 8000 — see the ship:spec:review budget comment above;
					// same undersized-budget root cause applies here.
					return pipe.InvokeChecked(
						"ship:spec:generate", "",
						specGenSystem,
						fmt.Sprintf("Generate a complete feature specification for: %s%s", description, wsSection),
						8000,
					)
				}
				// J8/J9: strip preamble and detect truncation before writing.
				generated, genComplete, genErr := generateWithValidation(genFn)
				if agentPauseCheckpoint(&cp, "ship:spec:generate", genErr) {
					return cp
				}
				switch {
				case genErr != nil:
					specContent = specStub(description)
					cp.Detail = fmt.Sprintf(
						"spec stub created (LLM:%s — %s): .forge/specs/%s/spec.md — edit before continuing",
						pipe.ProviderName(), llmErrNote(genErr), slug)
					appendFailure(root, "spec", description, cp.Detail+" | full: "+llmErrFull(genErr))
				case generated != "" && !genComplete:
					// J9: the exact incident that motivated this fix — a raw LLM
					// preamble sentence or a mid-Gherkin-block truncation must
					// never be written to spec.md as if it succeeded.
					specContent = specStub(description)
					cp.Detail = fmt.Sprintf(
						"spec generation truncated/incomplete after retry (LLM:%s) — stub created: "+
							".forge/specs/%s/spec.md — edit before continuing", pipe.ProviderName(), slug)
					appendFailure(root, "spec", description, cp.Detail)
				case generated != "":
					specContent = appendUnverifiedPathsWarning(root, generated)
					cp.Detail = fmt.Sprintf("spec generated by %s: .forge/specs/%s/spec.md",
						pipe.ProviderName(), slug)
				default:
					specContent = specStub(description)
					cp.Detail = fmt.Sprintf("spec stub created: .forge/specs/%s/spec.md — edit before continuing", slug)
				}
			} else {
				specContent = specStub(description)
				cp.Detail = fmt.Sprintf(
					"spec stub created: .forge/specs/%s/spec.md — edit before continuing "+
						"(run 'forge config set llm.provider <name>' or set ANTHROPIC_API_KEY / OPENAI_API_KEY)",
					slug)
			}
			if err := os.WriteFile(specFile, []byte(specContent), 0o600); err == nil {
				// G-005: write spec.yml alongside spec.md.
				writeSpecManifest(specsDir, slug, description, specContent)
				cp.Status = "ok"
				return cp
			}
		}
		// .forge/ not writable — still ok for dry-run; record the description.
		cp.Status = "ok"
		cp.Detail = fmt.Sprintf("description: %q (write .forge/specs/ to persist)", description)
		return cp
	}
	// No description — look for any existing spec.
	entries, err := os.ReadDir(specsDir)
	if err == nil && len(entries) > 0 {
		cp.Status = "ok"
		cp.Detail = fmt.Sprintf("%d spec(s) in .forge/specs/; pass a feature description to target one", len(entries))
		return cp
	}
	cp.Status = "warning"
	if pipe != nil {
		cp.Detail = fmt.Sprintf("no description; pass a feature description to generate a spec via %s",
			pipe.ProviderName())
	} else {
		cp.Detail = "no description and no specs in .forge/specs/; run: forge ship spec \"<your feature>\""
	}
	return cp
}

// slugify converts a free-text description to a filesystem-safe slug.
func slugify(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}

// checkTest verifies existing test files and (when an LLMPipe is available)
// generates failing test stubs via the configured LLM provider.
// specName, when non-empty, overrides the slug derived from description.
// M1-03: git timestamp guard — test files must predate or match their
// corresponding production files. If any prod file is newer than its test
// file by more than 60 seconds the checkpoint is marked as a failure.
//
// dryRun, when true, never writes to disk — matches the documented `--dry-run`
// contract ("preview what would happen without making LLM calls or git
// operations"). Even outside dry-run, the 4 named artifacts (G-006) are only
// ever scaffolded when at least one is missing: once all 4 exist, this
// function leaves them untouched rather than overwriting hand-written test
// content with fresh RED stubs on every run (see CHANGELOG 1.7.4 — a real
// bug where re-running `forge ship test`/`forge ship -d` clobbered completed
// test files back to placeholder stubs).
func checkTest(root, description, specName string, pipe *LLMPipe, dryRun bool) Checkpoint {
	cp := Checkpoint{Name: "Test"}
	testFiles := findTestFiles(root)

	// M1-03: tests-precede-code timestamp guard.
	if violations := testTimestampGuard(root); len(violations) > 0 {
		cp.Status = "fail"
		cp.Detail = fmt.Sprintf(
			"tests-precede-code violation: %d production file(s) modified without a corresponding test update — write/update tests first: %s",
			len(violations), strings.Join(violations[:min3(len(violations))], ", "),
		)
		// G-011: record this failure for future learning context.
		appendFailure(root, "test", description, cp.Detail)
		return cp
	}

	// Determine slug: --name/-n takes priority over the slug derived from description.
	slug := specName
	if slug == "" {
		slug = slugify(description)
	}

	// G-006: write the 4 named test artifacts, but only when scaffolding is
	// actually needed. Never write during --dry-run, and never overwrite a
	// slug whose 4 artifacts are already all present — writeTestArtifacts
	// unconditionally stomps existing files with RED placeholder stubs, so
	// re-running this checkpoint after real tests have been written must be a
	// no-op, not a regression.
	if description != "" && !dryRun && !allTestArtifactsExist(root, slug) {
		specMD := ""
		specPath := filepath.Join(root, ".forge", "specs", slug, "spec.md")
		if data, err := os.ReadFile(specPath); err == nil {
			specMD = string(data)
		}
		writeTestArtifacts(root, slug, description, specMD, pipe)
	}

	if len(testFiles) > 0 {
		cp.Status = "ok"
		missing := missingTestArtifacts(root, slug)
		if len(missing) == 0 {
			cp.Detail = fmt.Sprintf("%d test file(s) found; all 4 named artifacts present (tests/%s.*)", len(testFiles), slug)
		} else {
			cp.Detail = fmt.Sprintf("%d test file(s) found; missing artifacts: %s", len(testFiles), strings.Join(missing, ", "))
		}
		applyReachability(root, testFiles, &cp)
		if pipe != nil {
			if _, err := generateTestStubs(root, description, slug, pipe); err != nil {
				if agentPauseCheckpoint(&cp, "ship:test:generate", err) {
					return cp
				}
				cp.Detail += fmt.Sprintf(" [LLM:%s — %s]", pipe.ProviderName(), llmErrNote(err))
			}
		}
		return cp
	}
	// No test files — generate 4 named artifacts.
	if pipe != nil {
		if _, err := generateTestStubs(root, description, slug, pipe); err != nil {
			if agentPauseCheckpoint(&cp, "ship:test:generate", err) {
				return cp
			}
			cp.Status = "warning"
			cp.Detail = fmt.Sprintf("no test files; 4 artifacts written to tests/%s.* [LLM:%s — %s]",
				slug, pipe.ProviderName(), llmErrNote(err))
			return cp
		}
		cp.Status = "ok"
		cp.Detail = fmt.Sprintf("4 named test artifacts written to tests/%s.* — complete stubs before forge ship code", slug)
		return cp
	}
	cp.Status = "warning"
	cp.Detail = fmt.Sprintf("no test files found — 4 stub artifacts written to tests/%s.*", slug)
	if pipe == nil {
		cp.Detail += " (run 'forge config set llm.provider <name>' or set ANTHROPIC_API_KEY / OPENAI_API_KEY)"
	}
	applyReachability(root, findTestFiles(root), &cp)
	return cp
}

// applyReachability annotates the Test checkpoint with whether the project's
// runner will actually execute the test files on disk.
//
// It downgrades an "ok" checkpoint to "warning" when files are unreachable,
// but never fails the pipeline: making it blocking would break every project
// whose runner config forge cannot parse, and 1.7.12 already set the precedent
// that a new testing gate lands advisory first. Advisory is still a large
// improvement over the previous behaviour, which was to count the files and
// call it green — a report that was not merely incomplete but actively
// misleading, since an uncollected test looks exactly like a passing one.
func applyReachability(root string, files []string, cp *Checkpoint) {
	rep := verifyTestsReachable(root, files)
	if rep.Checked == 0 {
		return // nothing JS/TS to check — stay silent rather than add noise
	}
	if len(rep.Orphans) > 0 && cp.Status == "ok" {
		cp.Status = "warning"
	}
	cp.Detail += " | " + rep.Summary()
}

// testTimestampGuard returns production .go files that are dirty in the working
// tree without a corresponding dirty _test.go file. Scoping to the current
// working tree keeps the TDD gate relevant to the active change set; historical
// violations in already-committed files are not re-raised here.
// Returns nil when the working tree is clean (nothing new to check).
func testTimestampGuard(root string) []string {
	gs, err := gitservice.New(root)
	if err != nil {
		return nil // not a git repo — skip guard
	}
	statuses, err := gs.Status()
	if err != nil || len(statuses) == 0 {
		return nil // clean working tree — nothing to check for this change set
	}

	// Build a forward-slash set of changed paths; handle "old -> new" renames.
	dirty := make(map[string]bool, len(statuses))
	for _, st := range statuses {
		p := filepath.ToSlash(st.Path)
		if i := strings.Index(p, " -> "); i >= 0 {
			p = p[i+4:]
		}
		dirty[p] = true
	}

	var violations []string
	for rel := range dirty {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		testRel := strings.TrimSuffix(rel, ".go") + "_test.go"
		// No corresponding test file at all — not a timestamp violation (separate check).
		if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(testRel))); statErr != nil {
			continue
		}
		// Test file was also modified in the working tree — no violation.
		if dirty[testRel] {
			continue
		}
		violations = append(violations, filepath.FromSlash(rel))
	}
	return violations
}

// min3 returns min(n, 3) — used to cap violation list display.
func min3(n int) int {
	if n > 3 {
		return 3
	}
	return n
}

// findTestFiles counts *_test.go and *.test.ts files under root.
func findTestFiles(root string) []string {
	var out []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				n := d.Name()
				if n == ".git" || n == "node_modules" || n == "vendor" || n == "dist" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, "_test.go") ||
			strings.HasSuffix(name, ".test.ts") ||
			strings.HasSuffix(name, ".test.js") ||
			strings.HasSuffix(name, ".spec.ts") ||
			strings.HasSuffix(name, ".spec.js") {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// checkBreakdown looks for a task-breakdown file in .forge/specs/<slug>/ and,
// when an LLMPipe is available, generates one if it does not exist.
// specName, when non-empty, overrides the slug derived from description.
func checkBreakdown(root, description, specName string, pipe *LLMPipe) Checkpoint {
	cp := Checkpoint{Name: "Breakdown"}
	if description != "" || specName != "" {
		// Determine slug: --name/-n takes priority over slug derived from description.
		slug := specName
		if slug == "" {
			slug = slugify(description)
		}
		breakdownFile := filepath.Join(root, ".forge", "specs", slug, "breakdown.md")
		if _, err := os.Stat(breakdownFile); err == nil {
			cp.Status = "ok"
			cp.Detail = fmt.Sprintf("breakdown found: .forge/specs/%s/breakdown.md", slug)
			return cp
		}
		// Breakdown does not exist — attempt LLM generation.
		if pipe != nil {
			generated, err := generateBreakdown(root, description, slug, pipe)
			if agentPauseCheckpoint(&cp, "ship:breakdown:generate", err) {
				return cp
			}
			if err != nil {
				cp.Status = "warning"
				cp.Detail = fmt.Sprintf("no breakdown.md [LLM:%s — %s] — run forge ship breakdown to generate",
					pipe.ProviderName(), llmErrNote(err))
				// G-011: record breakdown failure for future learning context.
				// Persist the FULL error (not the truncated cp.Detail) so a
				// later diagnosis (human or LLM) can see the actual root
				// cause instead of a "..."-truncated fragment.
				appendFailure(root, "breakdown", description, cp.Detail+" | full: "+llmErrFull(err))
				return cp
			}
			if generated != "" {
				cp.Status = "ok"
				// G-008: write per-task context bundles.
				_ = writeTaskContextBundles(root, slug, 0)
				cp.Detail = fmt.Sprintf("breakdown+tasks+bundles by %s: .forge/specs/%s/",
					pipe.ProviderName(), slug)
				return cp
			}
		}
	}
	// Structural fallback.
	if _, err := os.Stat(filepath.Join(root, ".forge", "specs")); err == nil {
		cp.Status = "warning"
		cp.Detail = "no breakdown.md found — run forge ship breakdown to generate"
		if pipe == nil {
			cp.Detail += " (run 'forge config set llm.provider <name>' or set ANTHROPIC_API_KEY / OPENAI_API_KEY)"
		}
	} else {
		cp.Status = "warning"
		cp.Detail = "no .forge/specs/ directory — run forge ship spec first"
	}
	return cp
}

// checkCode verifies working-tree changes and (when an LLMPipe is available)
// generates a step-by-step code implementation plan from the spec+breakdown.
// specName, when non-empty, overrides the slug derived from description.
func checkCode(root, description, specName string, pipe *LLMPipe) Checkpoint {
	cp := Checkpoint{Name: "Code"}
	changedFiles := countChangedFiles(root)

	// Determine slug: --name/-n takes priority over slug derived from description.
	slug := specName
	if slug == "" {
		slug = slugify(description)
	}

	if pipe != nil {
		plan, err := generateCodePlan(root, description, slug, pipe)
		if agentPauseCheckpoint(&cp, "ship:code:generate", err) {
			return cp
		}
		if err != nil {
			if changedFiles > 0 {
				cp.Status = "ok"
				cp.Detail = fmt.Sprintf("%d modified file(s) [LLM:%s — %s]",
					changedFiles, pipe.ProviderName(), llmErrNote(err))
			} else {
				cp.Status = "warning"
				cp.Detail = fmt.Sprintf("no code changes detected [LLM:%s — %s]",
					pipe.ProviderName(), llmErrNote(err))
			}
			return cp
		}
		if plan != "" {
			if changedFiles > 0 {
				cp.Status = "ok"
				cp.Detail = fmt.Sprintf("%d modified file(s); code plan written by %s (see .forge/specs/%s/code-plan.md)",
					changedFiles, pipe.ProviderName(), slug)
			} else {
				cp.Status = "ok"
				cp.Detail = fmt.Sprintf("code plan written by %s (see .forge/specs/%s/code-plan.md) — implement then rerun",
					pipe.ProviderName(), slug)
			}
			return cp
		}
	}

	// G-009: auto-advance when all tasks complete.
	if description != "" && allTasksComplete(root, slug) {
		cp.Status = "ok"
		cp.AutoAdvance = true
		cp.Detail = "all tasks complete — auto-advancing to Ship checkpoint"
		return cp
	}

	// Structural fallback (no LLM or no spec/breakdown context).
	if changedFiles > 0 {
		cp.Status = "ok"
		cp.Detail = fmt.Sprintf("%d modified file(s) detected in working tree", changedFiles)
		return cp
	}
	cp.Status = "warning"
	cp.Detail = "no code changes detected in working tree — implement tasks then rerun forge ship code"
	if pipe == nil {
		cp.Detail += " (run 'forge config set llm.provider <name>' or set ANTHROPIC_API_KEY / OPENAI_API_KEY)"
	}
	return cp
}

// countChangedFiles returns the number of modified/untracked files via git status
// (equivalent to counting lines in `git status --porcelain`).
// Returns 0 if git is unavailable, the directory is not a repo, or the status
// call fails for any other reason.
//
// Prior implementation walked the entire working tree counting every
// .go/.ts/.js/.py/.sql file that existed on disk, regardless of whether it was
// actually changed — on a real project with thousands of source files this
// reported a number like "1693 modified file(s)" when the real change count
// (confirmed via `git status --short`) was in the single digits. That made the
// Code/Ship checkpoint output actively misleading. git status --porcelain
// respects .gitignore and reports only genuinely modified/staged/untracked
// paths, which is what the checkpoint detail message claims to report.
func countChangedFiles(root string) int {
	svc, err := gitservice.New(root)
	if err != nil {
		return 0
	}
	statuses, err := svc.Status()
	if err != nil {
		return 0
	}
	return len(statuses)
}

// checkVerify runs the security scanner, clean check, checks the manifest,
// and (TG-39) audits spec artefacts for incomplete tasks and authz gaps.
// M1-10: forge clean --check is now wired here.
// specName, when non-empty, overrides the slug derived from description.
func checkVerify(root, description, specName string, pipe *LLMPipe) Checkpoint {
	cp := Checkpoint{Name: "Ship"}

	// Run security scan.
	// G-022: assign confidence scores and only block ship on HIGH-confidence
	// findings (actual hardcoded secrets / credential leaks).  Medium-confidence
	// items (loose semver pins, admin-only SQL, etc.) and low-confidence items
	// (doc examples, test fixtures) are reported as warnings but do not block
	// the pipeline — they should be addressed via `forge scan security` in a
	// separate pass.
	scanRes, err := cmdscan.RunSecurity(root)
	if err != nil {
		cp.Status = "warning"
		cp.Detail = fmt.Sprintf("security scan error: %v", err)
		return cp
	}
	scanRes.Findings = cmdscan.AssignConfidence(scanRes.Findings)
	var highFindings []cmdscan.Finding
	for _, f := range scanRes.Findings {
		if f.Confidence == string(cmdscan.ConfidenceHigh) {
			highFindings = append(highFindings, f)
		}
	}
	if len(highFindings) > 0 {
		cp.Status = "fail"
		cp.Detail = fmt.Sprintf("security scan: %d high-confidence finding(s) — fix before shipping (run: forge scan security)", len(highFindings))
		return cp
	}

	// M1-10: forge clean --check — block ship if unmanaged scratch files exist.
	cleanRes, cleanErr := cmdclean.Run(root, false /* check mode */)
	if cleanErr != nil {
		cp.Status = "warning"
		cp.Detail = fmt.Sprintf("clean check error: %v — run `forge clean --check` manually", cleanErr)
		return cp
	}
	if len(cleanRes.Candidates) > 0 || len(cleanRes.TrackedSecrets) > 0 {
		detail := fmt.Sprintf("hygiene: %d unmanaged file(s)", len(cleanRes.Candidates))
		if len(cleanRes.TrackedSecrets) > 0 {
			detail += fmt.Sprintf("; %d tracked secret(s)", len(cleanRes.TrackedSecrets))
		}
		cp.Status = "fail"
		cp.Detail = detail + " — run `forge clean --apply` to remove, then re-run forge ship verify"
		return cp
	}

	// Check manifest.
	mf, _ := manifest.Load(filepath.Join(root, manifest.DefaultPath))
	patternCount := len(mf.Scratch) + len(mf.Managed)

	// TG-39: spec-vs-code audit — blocking gaps fail the checkpoint.
	// When a pipe is available, run the same auto-remediation loop used in
	// checkQAVerify so that both the Ship and QA-Verify checkpoints operate
	// consistently. The loop is capped at maxRemediationRounds.
	auditRes := auditSpecVsCode(root, description, specName)
	cp.GapAudit = &auditRes

	remediationRounds := 0
	remediationState := make(map[string]*RemediationState)
	for auditRes.HasBlockingGaps() && pipe != nil && remediationRounds < maxRemediationRounds {
		remediationRounds++
		// L4: Pass round number so shrinking context is used for round 2+.
		remediateGapsRound(root, description, specName, auditRes.Gaps, pipe, remediationRounds, remediationState)
		auditRes = auditSpecVsCode(root, description, specName)
		cp.GapAudit = &auditRes
	}

	if auditRes.HasBlockingGaps() {
		blocking := 0
		for _, g := range auditRes.Gaps {
			if g.Severity == "blocking" {
				blocking++
			}
		}
		cp.Status = "fail"
		cp.Detail = fmt.Sprintf(
			"spec audit: %d blocking gap(s) detected — fix before shipping (run: forge ship spec)",
			blocking,
		)
		return cp
	}

	cp.Status = "ok"
	// M1: the ship checkpoint has no post-checkpoint gates to earn its green
	// from, so it records its own. Unlike most "ok" assignments in this file,
	// these are real observations — the scanner and the hygiene checker were
	// actually run and actually answered.
	cp.AddEvidence(SourceExternalTool, "security scan found no high-confidence findings",
		fmt.Sprintf("%d finding(s) total, %d high-confidence", len(scanRes.Findings), len(highFindings)))
	cp.AddEvidence(SourceExternalTool, "hygiene check found no unmanaged files",
		fmt.Sprintf("%d manifest pattern(s)", patternCount))
	if auditRes.SpecFound {
		cp.AddEvidence(SourceReadBack, "spec-vs-code audit found no blocking gaps",
			fmt.Sprintf("%d warning-level gap(s)", len(auditRes.Gaps)))
	}

	warnCount := len(scanRes.Findings) - len(highFindings)
	if warnCount > 0 {
		cp.Detail = fmt.Sprintf("security scan: no high-confidence findings (%d medium/low advisory — run `forge scan security` to review); hygiene OK; manifest OK (%d patterns)", warnCount, patternCount)
	} else {
		cp.Detail = fmt.Sprintf("security scan clean; hygiene OK; manifest OK (%d patterns)", patternCount)
	}
	if auditRes.SpecFound && len(auditRes.Gaps) > 0 {
		// Warning-only gaps — note them but don't fail.
		cp.Detail += fmt.Sprintf("; %d spec audit warning(s)", len(auditRes.Gaps))
	}
	return cp
}

// checkQAVerify is checkpoint 7: quality-assurance verification by a QA/QE agent.
//
// It performs two distinct checks in sequence:
//
//  1. Spec-vs-code gap audit — re-runs auditSpecVsCode to verify that every
//     acceptance criterion in the spec has a corresponding implementation.
//     Blocking gaps (incomplete tasks, authz roles without RLS tests) cause an
//     immediate fail. Warning gaps (events not asserted) are reported in the
//     detail but do not stop the pipeline.
//
//  2. Test-runner probe — confirms the shipped artifact is functional:
//     MCP path (preferred):
//     Go project  — cmd/mcp/main.go present  → go test ./internal/mcpserver/...
//     Python proj — mcp_server.py present    → python -m pytest tests/test_mcp_server.py -q
//     Fallback (no MCP server):
//     Go project  — go.mod present           → go test ./... --count=1
//     Python proj — pyproject.toml/pytest.ini → pytest -q --tb=short
//     Node proj   — package.json w/ "test" script → npm test --silent
//     No runner   — warning (no tools configured)
//
// generateManualTestPlan produces a 6-role manual test plan written to
// .forge/specs/<slug>/manual-test-plan.md. Each role contributes one section
// via a targeted KB-enriched InvokeWithKnowledge call. A cross-role gap
// challenge (Round 2) surfaces coordination gaps between sections.
//
// Returns the absolute path written, or "" when pipe is nil or write fails.
func generateManualTestPlan(root, description string, pipe *LLMPipe) string {
	if pipe == nil {
		return ""
	}
	slug := slugify(description)
	planPath := filepath.Join(root, ".forge", "specs", slug, "manual-test-plan.md")

	// Load spec and arch for context.
	specContent, _ := os.ReadFile(filepath.Join(root, ".forge", "specs", slug, "spec.md"))
	archContent, _ := os.ReadFile(filepath.Join(root, ".forge", "specs", slug, "arch.md"))
	featureContext := fmt.Sprintf(
		"Feature: %s\n\nSpec:\n%s\n\nArchitecture notes:\n%s",
		description, string(specContent), string(archContent),
	)

	type roleSection struct {
		opKey     string   // operation key for InvokeWithKnowledge
		heading   string   // Markdown heading for this section
		kbFamily  string   // KB family for InvokeWithKnowledge
		kbTags    []string // KB tags for InvokeWithKnowledge
		sysPrompt string   // role-specific system prompt
	}

	sections := []roleSection{
		{
			opKey:    "qa-verify:manual:product-owner",
			heading:  "## Product Owner — User Acceptance Tests",
			kbFamily: "",
			kbTags:   []string{"requirements", "acceptance-criteria", "user-stories"},
			sysPrompt: "You are a Product Owner writing user acceptance tests. " +
				"Provide step-by-step UAT walkthroughs verifying business value. " +
				"Write scenarios executable by a non-technical stakeholder: numbered steps, exact data inputs, expected results. " +
				"Cover: happy path, main alternative path, and one rejection/undo path per AC item.",
		},
		{
			opKey:    "qa-verify:manual:business-analyst",
			heading:  "## Business Analyst — Business Rule & Edge Case Tests",
			kbFamily: "",
			kbTags:   []string{"edge-cases", "state-transitions", "validation", "business-rules"},
			sysPrompt: "You are a Business Analyst writing manual test scenarios for business rules. " +
				"Cover: state-machine transitions, concurrent-update edge cases, validation boundaries (min/max/null), " +
				"and cross-field dependency rules. Provide exact input values and expected outcomes for each scenario.",
		},
		{
			opKey:    "qa-verify:manual:quality-engineer",
			heading:  "## Quality Engineer — Full Test Spectrum",
			kbFamily: "",
			kbTags: []string{
				"test-design", "quality-gate", "tdd",
				"integration-testing", "e2e", "regression",
				"contract-testing", "mutation-testing", "smoke-testing",
				"acceptance-testing", "exploratory-testing",
				"performance-testing", "test-pyramid",
			},
			sysPrompt: "You are a senior Quality Engineer writing a full-spectrum manual test plan. " +
				"Cover ALL test types: smoke (post-deploy checklist), exploratory (session-based), " +
				"performance sanity (response-time vs spec NFR thresholds), mutation risk areas " +
				"(list code paths most likely to break on trivial edits), regression guards " +
				"(prior known defects to re-verify), and contract verification (API schema diffs). " +
				"Group by test type with explicit preconditions and verdict criteria.",
		},
		{
			opKey:    "qa-verify:manual:security-reviewer",
			heading:  "## Security Reviewer — Security Probes",
			kbFamily: "security",
			kbTags:   []string{"owasp", "authz", "injection", "audit-logging", "rls", "privilege-escalation"},
			sysPrompt: "You are a Security Reviewer writing security probe test scenarios. " +
				"Reference OWASP Top 10. For each relevant risk provide: exact probe steps, " +
				"crafted payloads (safe — no real exploits), and expected outcome (BLOCKED / LOGGED). " +
				"Mandatory probes: (A01) broken access control attempt, (A02) injection in all input fields, " +
				"(A07) auth bypass attempt, (A09) audit trail completeness check, " +
				"(A10) server-side request forgery if external URLs are involved.",
		},
		{
			opKey:    "qa-verify:manual:devops-sre",
			heading:  "## DevOps / SRE — Operational Readiness Tests",
			kbFamily: "reliability",
			kbTags:   []string{"observability", "rollback", "health-check", "slo", "deployment", "circuit-breaker"},
			sysPrompt: "You are a DevOps/SRE writing operational readiness test scenarios. " +
				"Include: health-check endpoints respond correctly post-deploy, " +
				"structured logs and traces emitted for new code paths, " +
				"rollback procedure (step-by-step) completes within SLO, " +
				"graceful degradation under load (circuit-breaker trip verified), " +
				"and alerting rules fire correctly on injected failures.",
		},
		{
			opKey:    "qa-verify:manual:compliance-officer",
			heading:  "## Compliance & Privacy Officer — Compliance Attestation Tests",
			kbFamily: "compliance",
			kbTags:   []string{"pci-dss", "gdpr", "audit", "data-residency", "pii", "consent"},
			sysPrompt: "You are a Compliance & Privacy Officer writing compliance attestation test scenarios. " +
				"Reference applicable regulations by article number (e.g. GDPR Art. 25 data-by-design). " +
				"Cover: PII fields masked/tokenised in logs and error messages, " +
				"audit-trail entry created for every sensitive operation, " +
				"data-residency constraints respected (no cross-region leakage), " +
				"consent management flows work and are reversible, " +
				"data-retention and right-to-erasure flows tested end-to-end.",
		},
	}

	var planParts []string
	planParts = append(planParts, fmt.Sprintf(
		"# Manual Test Plan: %s\n\n_Generated by `forge ship qa-verify` — %s_\n\n---\n",
		description, time.Now().UTC().Format("2006-01-02"),
	))
	planParts = append(planParts,
		"## Preconditions\n\n"+
			"- [ ] Test environment is provisioned and seeded with representative test data\n"+
			"- [ ] All automated tests are GREEN before manual testing begins\n"+
			"- [ ] Tester has the required roles/permissions configured (use placeholder credentials)\n"+
			"- [ ] `spec.md`, `arch.md`, and this plan are accessible to all testers\n\n---\n",
	)

	// Round 1: each role generates their independent section.
	for _, s := range sections {
		resp, err := pipe.InvokeWithKnowledge(
			s.opKey, "", s.sysPrompt, featureContext, 800,
			"qa-verify", s.kbFamily, "", s.kbTags,
		)
		if err != nil || resp == "" {
			resp = fmt.Sprintf(
				"_Section not generated — check LLM configuration (`forge config set llm.provider <name>`)._",
			)
		}
		planParts = append(planParts, s.heading+"\n\n"+resp+"\n\n---\n")
	}

	// Round 2: cross-role gap challenge — QA Lead surfaces coordination gaps.
	combined := strings.Join(planParts, "")
	gapResp, _ := pipe.Invoke(
		"qa-verify:manual:cross-challenge", "",
		"You are a QA Lead reviewing a multi-role manual test plan for completeness. "+
			"Identify: "+
			"(1) AC items from the spec not covered by any role's section; "+
			"(2) scenarios only partially covered (one role handles step A but no one handles step B); "+
			"(3) coordination requirements between sections (e.g. Security probe requires Ops rollback ready first). "+
			"Output ONLY a concise '## Cross-Role Gap Analysis' section. Be specific — list gap, owner, and remediation.",
		combined, 500,
	)
	if gapResp != "" {
		planParts = append(planParts, gapResp+"\n")
	}

	content := strings.Join(planParts, "")
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		return ""
	}
	if err := os.WriteFile(planPath, []byte(content), 0o600); err != nil {
		return ""
	}
	return planPath
}

// runQATestSuite executes the appropriate automated test suite and returns
// (status, detail). Called exclusively by checkQAVerify so that the manual
// test plan generation can run once after all automated tests complete.
func runQATestSuite(root string) (status, detail string) {
	sp := procspawn.New("go", "python", "python3", "pytest", "npm")

	goMCPEntry := filepath.Join(root, "cmd", "mcp", "main.go")
	pyMCPEntry := filepath.Join(root, "mcp_server.py")
	goModFile := filepath.Join(root, "go.mod")

	// Node/TypeScript projects have no standardized MCP-entry-point
	// convention analogous to cmd/mcp/main.go or mcp_server.py, so this is a
	// native-fallback-only detection: package.json must exist AND declare a
	// non-empty "test" script. `npm test` with no script defined just prints
	// an npm error and exits 1 — indistinguishable from a real test failure —
	// so the script's presence is checked up front rather than letting a
	// missing script masquerade as a failing test suite.
	hasNpmTestScript := false
	if data, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		var pkg struct {
			Scripts map[string]string `json:"scripts"`
		}
		if json.Unmarshal(data, &pkg) == nil && strings.TrimSpace(pkg.Scripts["test"]) != "" {
			hasNpmTestScript = true
		}
	}

	switch {
	case pathExists(goMCPEntry):
		res, err := sp.Run("go",
			[]string{"test", "./internal/mcpserver/...", "-count=1", "-timeout=120s"},
			procspawn.Options{Dir: root, Timeout: 130 * time.Second},
		)
		if err != nil {
			return "fail", fmt.Sprintf("QA (MCP/go): mcpserver tests failed — %v", err)
		}
		passed := strings.Count(res.Stdout+res.Stderr, "--- PASS")
		if passed == 0 {
			passed = strings.Count(res.Stdout, "ok")
		}
		return "ok", fmt.Sprintf(
			"QA via MCP (go): mcpserver unit tests passed (%d case(s) confirmed, %.1fs) — AI-agent tools operational",
			passed, res.Duration.Seconds())

	case pathExists(pyMCPEntry):
		runner, runArgs := "python3", []string{"-m", "pytest", "tests/test_mcp_server.py", "-q", "--tb=short"}
		res, err := sp.Run(runner, runArgs, procspawn.Options{Dir: root, Timeout: 130 * time.Second})
		if err != nil && strings.Contains(err.Error(), "not in the spawn allow-list") {
			runner = "python"
			res, err = sp.Run(runner, runArgs, procspawn.Options{Dir: root, Timeout: 130 * time.Second})
		}
		if err != nil {
			return "fail", fmt.Sprintf("QA (MCP/python): test_mcp_server.py failed — %v", err)
		}
		passed := strings.Count(res.Stdout, " passed")
		return "ok", fmt.Sprintf(
			"QA via MCP (python): test_mcp_server.py passed (%d case(s) confirmed, %.1fs) — AI-agent tools operational",
			passed, res.Duration.Seconds())

	case pathExists(goModFile):
		res, err := sp.Run("go",
			[]string{"test", "./...", "-count=1", "-timeout=180s"},
			procspawn.Options{Dir: root, Timeout: 190 * time.Second},
		)
		if err != nil {
			return "warning", fmt.Sprintf("QA (go test ./...): tests did not pass (%v) — add cmd/mcp/ for MCP-based QA", err)
		}
		pkgs := strings.Count(res.Stdout+res.Stderr, "\nok ")
		return "ok", fmt.Sprintf(
			"QA (go test ./...): %d package(s) passed (%.1fs) — add cmd/mcp/ to enable MCP agent QA",
			pkgs, res.Duration.Seconds())

	case pathExists(filepath.Join(root, "pyproject.toml")) ||
		pathExists(filepath.Join(root, "pytest.ini")) ||
		pathExists(filepath.Join(root, "setup.cfg")):
		runner := "python3"
		res, err := sp.Run(runner,
			[]string{"-m", "pytest", "-q", "--tb=short"},
			procspawn.Options{Dir: root, Timeout: 190 * time.Second},
		)
		if err != nil && strings.Contains(err.Error(), "not in the spawn allow-list") {
			runner = "python"
			res, err = sp.Run(runner,
				[]string{"-m", "pytest", "-q", "--tb=short"},
				procspawn.Options{Dir: root, Timeout: 190 * time.Second},
			)
		}
		if err != nil {
			return "warning", fmt.Sprintf("QA (pytest): tests did not pass (%v) — add mcp_server.py for MCP agent QA", err)
		}
		passed := strings.Count(res.Stdout, " passed")
		return "ok", fmt.Sprintf(
			"QA (pytest): %d case(s) passed (%.1fs) — add mcp_server.py to enable MCP agent QA",
			passed, res.Duration.Seconds())

	case hasNpmTestScript:
		res, err := sp.Run("npm",
			[]string{"test", "--silent"},
			procspawn.Options{Dir: root, Timeout: 190 * time.Second},
		)
		if err != nil {
			return "warning", fmt.Sprintf("QA (npm test): tests did not pass (%v) — add mcp_server.py/cmd/mcp for MCP agent QA", err)
		}
		out := res.Stdout + res.Stderr
		passed := 0
		// Jest's summary line isn't always "Tests: N passed" — a preceding
		// "N todo," or "N failed," group is common (e.g. "Tests: 6 todo,
		// 4118 passed, 4124 total"), so anchoring on "Tests:" immediately
		// before the digits misses the real passed-count. But matching
		// "N passed" anywhere in the output is also wrong: Jest always
		// prints a "Test Suites: N passed, N total" line BEFORE the
		// "Tests: ..." line, so an unanchored search finds the suite count,
		// not the individual-test count. Anchor to a line that starts with
		// "Tests:" specifically (not "Test Suites:") and take the first
		// "N passed" within that line.
		if m := regexp.MustCompile(`(?m)^Tests:.*?(\d+)\s+passed`).FindStringSubmatch(out); m != nil {
			passed, _ = strconv.Atoi(m[1])
		}
		return "ok", fmt.Sprintf(
			"QA (npm test): %d case(s) passed (%.1fs) — add mcp_server.py/cmd/mcp to enable MCP agent QA",
			passed, res.Duration.Seconds())
	}

	// No known test runner found.
	return "warning", ""
}

// specName, when non-empty, overrides the slug derived from description.
func checkQAVerify(root, description, specName string, pipe *LLMPipe) Checkpoint {
	cp := Checkpoint{Name: "QA-Verify"}

	// ── Phase 1: spec-vs-code gap audit with auto-remediation loop ───────────
	// Re-run the same audit that checkpoint 6 runs so that qa-verify is
	// self-contained. When an LLM is available and blocking gaps are found, the
	// loop calls remediateGaps to implement the missing pieces, then re-audits.
	// This continues until all blocking gaps are cleared or maxRemediationRounds
	// is exhausted — ensuring the pipeline ships only when fully spec-compliant.
	auditRes := auditSpecVsCode(root, description, specName)
	cp.GapAudit = &auditRes

	remediationState := make(map[string]*RemediationState)
	for auditRes.HasBlockingGaps() && pipe != nil && cp.RemediationRounds < maxRemediationRounds {
		cp.RemediationRounds++
		// L4: Pass round number so shrinking context is used for round 2+.
		remediateGapsRound(root, description, specName, auditRes.Gaps, pipe, cp.RemediationRounds, remediationState)
		auditRes = auditSpecVsCode(root, description, specName)
		cp.GapAudit = &auditRes
	}

	if auditRes.HasBlockingGaps() {
		blocking := 0
		for _, g := range auditRes.Gaps {
			if g.Severity == "blocking" {
				blocking++
			}
		}
		cp.Status = "fail"
		if cp.RemediationRounds > 0 {
			cp.Detail = fmt.Sprintf(
				"QA spec audit: %d blocking gap(s) remain after %d remediation round(s) — manual fix required",
				blocking, cp.RemediationRounds,
			)
		} else {
			cp.Detail = fmt.Sprintf(
				"QA spec audit: %d blocking gap(s) — no LLM configured; fix manually (run: forge ship spec to review)",
				blocking,
			)
		}
		appendFailure(root, "qa-verify", description, cp.Detail)
		return cp
	}

	// ── Phase 2: automated test suite ───────────────────────────────────────
	cp.Status, cp.Detail = runQATestSuite(root)

	// Append spec audit warnings to detail regardless of runner.
	if auditRes.SpecFound && len(auditRes.Gaps) > 0 {
		cp.Detail += fmt.Sprintf("; %d spec audit warning(s)", len(auditRes.Gaps))
	}

	// Hard-stop on test failure before generating the manual plan.
	if cp.Status == "fail" {
		appendFailure(root, "qa-verify", description, cp.Detail)
		return cp
	}

	// No test runner found — warn with actionable detail, skip manual plan.
	if cp.Status == "warning" && cp.Detail == "" {
		if auditRes.SpecFound && len(auditRes.Gaps) > 0 {
			cp.Detail = fmt.Sprintf("QA-Verify: no test runner found; %d spec audit warning(s) — "+
				"add cmd/mcp/ (Go) or mcp_server.py (Python) for AI-agent QA, "+
				"or ensure go.mod / pyproject.toml / package.json (with a \"test\" script) is present for native test fallback",
				len(auditRes.Gaps))
		} else {
			cp.Detail = "QA-Verify: no MCP server or test runner found — " +
				"add cmd/mcp/ (Go) or mcp_server.py (Python) for AI-agent QA, " +
				"or ensure go.mod / pyproject.toml / package.json (with a \"test\" script) is present for native test fallback"
		}
		return cp
	}

	// ── Phase 3: 6-role manual test plan generation ──────────────────────────
	// Runs only when automated tests pass (status == "ok") and an LLM is
	// available. The plan is written to .forge/specs/<slug>/manual-test-plan.md.
	if cp.Status == "ok" {
		if planPath := generateManualTestPlan(root, description, pipe); planPath != "" {
			cp.Detail += "; manual test plan: " + planPath
		}
	}

	return cp
}

// pathExists returns true when path exists (any file/dir type).
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// RunCheckpoints executes the requested checkpoints (nil = all five) in YOLO mode
func RunCheckpoints(root, description string, names []string) *ShipResult {
	return runWithOptions(RunOptions{Root: root, Description: description, Names: names})
}

// RunCheckpointsGated executes the requested checkpoints with an optional approval
// gate between steps. A nil gate is equivalent to YOLO (no prompts, no debate).
func RunCheckpointsGated(root, description string, names []string, gate Gate) *ShipResult {
	return runWithOptions(RunOptions{Root: root, Description: description, Names: names, Gate: gate})
}

// RunWithOptions is the primary execution entry point supporting all options including
// the self-debate engine (set DebateOpts to enable).
func RunWithOptions(opts RunOptions) *ShipResult {
	return runWithOptions(opts)
}

// runWithOptions is the internal implementation.
func runWithOptions(opts RunOptions) *ShipResult {
	root := opts.Root

	// Initialize the LLM pipe once for the entire run.
	// opts.LLMPipe enables test injection of a MockProvider; production code
	// leaves it nil so auto-detection runs via llmprovider.DetectOrPrompt (or
	// silent dry-run when --dry-run is active).
	pipe := opts.LLMPipe
	switch {
	case opts.AgentBridge != nil:
		// Agent mode wins over an injected pipe and over auto-detection: the
		// user explicitly asked for the host agent to do the reasoning, and
		// silently billing a detected API key instead would be the exact
		// surprise this mode exists to prevent.
		pipe = newLLMPipeAgent(root, opts.AgentBridge)
	case pipe == nil:
		pipe = newLLMPipeInteractive(root, opts.DryRun)
	}

	// Resolve quality-gate hooks and config.
	// opts.Hooks enables test injection; production code uses defaultHooks().
	hooks := opts.Hooks
	if hooks == nil {
		hooks = defaultHooks()
	}
	hookCfg := loadHookConfig(root)
	// Strict testing is on by default as of 1.8.2. Precedence, tightest last:
	//
	//	default (on) → .forge/hooks.yaml → --strict-testing → --no-strict-testing
	//
	// --strict-testing survives as a no-op that can still force the gate on
	// over a project file that turned it off; it is kept because scripts and
	// CI jobs in the wild pass it, and having it start erroring would break
	// them to no purpose. --no-strict-testing is the only way to turn the gate
	// off from the CLI, and it is checked last so an explicit opt-out always
	// wins over an explicit opt-in.
	hookCfg.StrictTesting = hookCfg.StrictTesting || opts.StrictTesting
	if opts.NoStrictTesting {
		hookCfg.StrictTesting = false
	}

	// P1: load domain profile for per-checkpoint budget/steering overrides.
	domainProfile := LoadDomainProfile(root, opts.DomainProfileName)

	// Derive the spec slug once so snapshot paths are consistent.
	specSlug := opts.SpecName
	if specSlug == "" && opts.Description != "" {
		specSlug = slugify(opts.Description)
	}

	// P2: take a snapshot before each checkpoint so that failures can be rolled back.
	// Errors are warnings only — a snapshot failure must never block the pipeline.
	snapBefore := func(cpName string) {
		if specSlug != "" {
			if err := TakeSnapshot(root, specSlug, cpName); err != nil {
				// Non-fatal: log to stderr and continue. The snapshot is best-effort.
				_, _ = fmt.Fprintf(os.Stderr, "forge ship: snapshot warning (cp=%s): %v\n", cpName, err)
			}
		}
	}
	// P2: restore the pre-checkpoint snapshot when a checkpoint fails.
	// Provides all-or-nothing semantics for the spec artefacts directory.
	snapOnFail := func(cpName string) {
		if specSlug != "" {
			if err := RestoreSnapshot(root, specSlug, cpName); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "forge ship: restore-snapshot warning (cp=%s): %v\n", cpName, err)
			}
		}
	}

	// P2: write a TrashManifest so `forge undo` can locate this ship run.
	shipRunID := fmt.Sprintf("ship-%s-%d", specSlug, time.Now().UnixMilli())
	writeShipTrashManifest(root, shipRunID, specSlug)

	// P2: start a pipeline-level telemetry span for OTEL-compatible tracing.
	pipeTraceID, pipeSpanID := telemetry.StartPipelineSpan(root, "ship")

	// Suppress the "unused" warning for domainProfile when no checkpoint reads it yet.
	_ = domainProfile

	// J10: only invoke the checkpoint(s) actually requested. A single-checkpoint
	// subcommand (e.g. `forge ship spec "..."`) must not silently execute every
	// other checkpoint's real LLM calls and file writes just because the
	// reporting step filters them out afterward — that used to be exactly what
	// happened here: every check* function ran unconditionally, and opts.Names
	// only trimmed the *displayed* result, not the work already done.
	runAll := len(opts.Names) == 0
	want := make(map[string]bool, len(opts.Names))
	for _, n := range opts.Names {
		want[n] = true
	}
	if want["verify"] {
		want["ship"] = true // G-003: deprecated alias for the same checkpoint
	}
	needs := func(name string) bool { return runAll || want[name] }

	results := make(map[string]Checkpoint, 7)

	// Agent mode serialises the run and stops at the first pause. Both are
	// consequences of the host agent being a single conversation:
	//   - the bridge is driven from one goroutine, so the arch/test DAG below
	//     must not fan out;
	//   - once a turn is owed there is nothing further to compute until it is
	//     answered, so continuing would only queue up work to be redone on
	//     the next replay.
	agentPaused := func() bool { return opts.AgentBridge.Paused() }
	serial := opts.AgentBridge != nil

	// Pre-checkpoint hooks. Until now this phase was declared, documented, and
	// never invoked — self-review-gate has been counted among forge's quality
	// gates since it was written without ever executing once.
	//
	// Findings are stashed rather than acted on immediately: the phase runs
	// before the checkpoint exists, so there is no Checkpoint to annotate yet.
	// They are attached in the reporting loop below, where the result is.
	preHookNotes := map[string][]HookResult{}
	runPre := func(name string) {
		if len(hooks) == 0 {
			return
		}
		res := runHooks(PhasePreCheckpoint, HookContext{
			Phase:          PhasePreCheckpoint,
			CheckpointName: name,
			Root:           root,
			Description:    opts.Description,
			SpecName:       opts.SpecName,
			Pipe:           pipe,
			Result:         nil, // by definition — the checkpoint has not run
			StrictTesting:  hookCfg.StrictTesting,
		}, hooks, hookCfg)
		if len(res) > 0 {
			preHookNotes[name] = res
		}
	}
	// beforeCheckpoint bundles the three things that must happen before every
	// checkpoint, so a new checkpoint cannot pick up two of them and silently
	// miss the third.
	beforeCheckpoint := func(name string) {
		snapBefore(name)
		pipe.SetCheckpoint(name)
		runPre(name)
	}

	if needs("spec") {
		beforeCheckpoint("spec")
		results["spec"] = checkSpec(root, opts.Description, opts.SpecName, pipe)
	}

	// P1 DAG: run arch and test in parallel when both are needed — they are
	// independent given spec. Each still runs standalone (reading spec.md /
	// prior artefacts from disk) when only one of them was requested.
	runArch := needs("arch") && !agentPaused()
	runTest := needs("test") && !agentPaused()
	var archCP, testCP Checkpoint
	var dagWG sync.WaitGroup
	switch {
	case serial:
		if runArch {
			beforeCheckpoint("arch")
			archCP = checkArch(root, opts.Description, opts.SpecName, pipe)
		}
		if runTest && !agentPaused() {
			beforeCheckpoint("test")
			testCP = checkTest(root, opts.Description, opts.SpecName, pipe, opts.DryRun)
		} else {
			runTest = false
		}
	default:
		if runArch {
			dagWG.Add(1)
			beforeCheckpoint("arch")
			go func() {
				defer dagWG.Done()
				archCP = checkArch(root, opts.Description, opts.SpecName, pipe)
			}()
		}
		if runTest {
			dagWG.Add(1)
			beforeCheckpoint("test")
			go func() {
				defer dagWG.Done()
				testCP = checkTest(root, opts.Description, opts.SpecName, pipe, opts.DryRun)
			}()
		}
		dagWG.Wait()
	}
	if runArch {
		results["arch"] = archCP
	}
	if runTest {
		results["test"] = testCP
	}

	if needs("breakdown") && !agentPaused() {
		beforeCheckpoint("breakdown")
		results["breakdown"] = checkBreakdown(root, opts.Description, opts.SpecName, pipe)
	}
	if needs("code") && !agentPaused() {
		beforeCheckpoint("code")
		results["code"] = checkCode(root, opts.Description, opts.SpecName, pipe)
	}
	if needs("ship") && !agentPaused() {
		beforeCheckpoint("ship")
		results["ship"] = checkVerify(root, opts.Description, opts.SpecName, pipe)
	}
	if needs("qa-verify") && !agentPaused() {
		beforeCheckpoint("qa-verify")
		results["qa-verify"] = checkQAVerify(root, opts.Description, opts.SpecName, pipe)
	}
	// PR checkpoint: appended only for full-pipeline runs with --pr.
	if opts.CreatePR && runAll {
		results["pr"] = checkPR(root, opts.Description, opts.SpecName)
	}

	canonicalOrder := []string{"spec", "arch", "test", "breakdown", "code", "ship", "qa-verify", "pr"}
	knownCheckpoint := map[string]bool{
		"spec": true, "arch": true, "test": true, "breakdown": true,
		"code": true, "ship": true, "verify": true, "qa-verify": true,
	}

	var selected []Checkpoint
	if runAll {
		for _, n := range canonicalOrder {
			if cp, ok := results[n]; ok {
				selected = append(selected, cp)
			}
		}
	} else {
		for _, n := range opts.Names {
			lookup := n
			if n == "verify" {
				lookup = "ship" // G-003: deprecated alias
			}
			if !knownCheckpoint[n] {
				selected = append(selected, Checkpoint{
					Name:   n,
					Status: "fail",
					Detail: fmt.Sprintf("unknown checkpoint %q; one of: spec, arch, test, breakdown, code, ship", n),
				})
				continue
			}
			selected = append(selected, results[lookup])
		}
	}

	res := &ShipResult{
		DryRun:        opts.DryRun,
		DebateEnabled: opts.DebateOpts != nil,
		Message:       shipMessage(pipe),
		Complexity:    classifyComplexity(opts.Description, root),
	}
	// P1: wire the complexity tier into the LLM pipe so that the tier router
	// selects the right model (T0/T1/T2) based on task complexity.
	pipe.SetComplexityTier(res.Complexity)

	total := len(selected)

	for i, cp := range selected {
		// A checkpoint that deferred to a host-agent turn produced no
		// artefact — none of hooks, evidence policy, digesting, or the
		// completion marker have anything real to inspect, and running them
		// anyway is actively harmful (the completion marker in particular
		// would write placeholder content into the checkpoint's own artefact
		// path). The outer `forge ship` command renders the pending turn from
		// bridge state regardless of what res contains, so it is safe to stop
		// here without evaluating the rest of the loop body for this entry.
		if cp.AgentPaused {
			res.Checkpoints = append(res.Checkpoints, cp)
			return res
		}
		// ── Post-checkpoint quality-gate hooks ───────────────────────────────
		// Hooks run after the check* function and can annotate cp.Detail with
		// warnings or escalate status to "fail" when HookConfig.Strict is set.
		if len(hooks) > 0 {
			hookCtx := HookContext{
				Phase:          PhasePostCheckpoint,
				CheckpointName: strings.ToLower(cp.Name),
				Root:           root,
				Description:    opts.Description,
				SpecName:       opts.SpecName,
				Pipe:           pipe,
				Result:         &cp,
				StrictTesting:  hookCfg.StrictTesting,
			}
			// Pre-checkpoint findings are merged with the post-checkpoint ones
			// so both phases reach the same reporting and escalation rules.
			// Keeping them on separate paths is how the pre-checkpoint phase
			// stayed unwired and unnoticed for as long as it did.
			allResults := runHooks(PhasePostCheckpoint, hookCtx, hooks, hookCfg)
			allResults = append(preHookNotes[strings.ToLower(cp.Name)], allResults...)
			failures, unverified := partitionResults(allResults)

			// Gates that could not check are reported separately and never
			// escalate the checkpoint. They are not evidence of a problem —
			// but they are also not evidence of correctness, and reporting a
			// checkpoint as clean on the strength of checks that never ran is
			// the failure this whole distinction exists to end.
			//
			// Suppressed on an already-failed checkpoint: that run has a real
			// error to show, and burying it under "unverified" notes for gates
			// that were moot anyway helps nobody.
			if len(unverified) > 0 && cp.Status != "fail" {
				var notes []string
				for _, u := range unverified {
					notes = append(notes, u.Message)
				}
				cp.Detail += " | UNVERIFIED[" + strings.Join(notes, "; ") + "]"
			}

			if len(failures) > 0 {
				// four-stage-testing-gate only ever fails when StrictTesting
				// is on (see testing_pipeline.go), so its failure must
				// escalate the checkpoint even when the unrelated global
				// hookCfg.Strict is off — but escalating THIS way must not
				// also promote some other, unrelated same-checkpoint hook
				// failure (e.g. manual-test-plan-gate) that only
				// hookCfg.Strict is supposed to govern. Check by hook name,
				// not just "StrictTesting is on somewhere in this run".
				strictTestingFailure := false
				for _, f := range failures {
					cp.Detail += " | HOOK[" + f.Message + "]"
					if f.HookName == fourStageTestingGate.Name {
						strictTestingFailure = true
					}
				}
				if (hookCfg.Strict || (hookCfg.StrictTesting && strictTestingFailure)) && cp.Status != "fail" {
					cp.Status = "fail"
				} else if cp.Status == "ok" {
					cp.Status = "warning"
				}
			}
		}

		// M1: a green checkpoint has to rest on something other than forge
		// reporting its own success. Runs last, after every gate has had the
		// chance to contribute evidence, so it only fires when genuinely
		// nothing independent was observed.
		applyEvidencePolicy(&cp)
		// P1-L2: write checkpoint digest on success for downstream context compression.
		// J6 (fix-checkpoint-llm-quality-and-observability): digest from the
		// real generated artefact, not cp.Detail (the one-line status
		// message) — confirmed dogfooding this that Detail-derived digests
		// showed token_estimate:~19 and empty decisions/constraints even on a
		// multi-KB, fully successful generation, making the digest useless
		// for auditing whether a checkpoint actually engaged the LLM.
		if cp.Status != "fail" && specSlug != "" {
			digestSource := checkpointArtefactContent(root, specSlug, cp.Name)
			if digestSource == "" {
				digestSource = cp.Detail
			}
			dig := makeDigestFromArtefact(strings.ToLower(cp.Name), digestSource)
			writeCheckpointDigest(root, specSlug, dig)
		}

		// Write a completion marker <checkpoint>.md to .forge/specs/<slug>/ so that
		// `forge ship status` can count this checkpoint as done.
		// spec.md / arch.md / breakdown.md are already written by their check* functions;
		// test.md, code.md, ship.md, qa-verify.md are not — write them here.
		if cp.Status != "fail" && specSlug != "" {
			cpLowerName := strings.ToLower(cp.Name)
			markerPath := filepath.Join(root, ".forge", "specs", specSlug, cpLowerName+".md")
			if _, statErr := os.Stat(markerPath); os.IsNotExist(statErr) {
				// M1: the marker is the durable record — what `forge ship
				// status` reads and what a human opens months later to ask
				// "was this actually checked?". Recording the status without
				// its basis leaves that question unanswerable, which is the
				// whole failure this work is about.
				basis := cp.EvidenceSummary()
				if basis == "" {
					basis = "none — no independent verification was recorded for this checkpoint"
				}
				marker := fmt.Sprintf(
					"# %s checkpoint\n\nStatus: %s\nCompleted: %s\nEvidence: %s\n\n%s\n",
					cp.Name, cp.Status, time.Now().UTC().Format(time.RFC3339), basis, cp.Detail)
				_ = os.WriteFile(markerPath, []byte(marker), 0o600)
			}
		}

		// Hard stop on failure regardless of gate.
		if cp.Status == "fail" {
			res.Checkpoints = append(res.Checkpoints, cp)
			res.Ready = false
			res.Message = fmt.Sprintf("checkpoint %s failed; pipeline stopped", cp.Name)
			// P2: restore the pre-checkpoint snapshot so the spec artefacts
			// directory is rolled back to the state before this checkpoint ran.
			snapOnFail(strings.ToLower(cp.Name))
			// P2: emit an ERROR checkpoint span for OTEL-compatible tracing.
			_ = telemetry.EmitCheckpointSpan(root, pipeTraceID, pipeSpanID, strings.ToLower(cp.Name), "ERROR", 0)
			// TG-40: emit gap.detected events (for ship/verify) before ship.failed.
			if opts.EventWriter != nil {
				cpLower := strings.ToLower(cp.Name)
				if (cpLower == "ship" || cpLower == "verify" || cpLower == "qa-verify") && cp.GapAudit != nil {
					for _, g := range cp.GapAudit.Gaps {
						gapCP := Checkpoint{
							Name:   cp.Name,
							Status: g.Severity,
							Detail: fmt.Sprintf("[%s] %s — hint: %s", g.Type, g.Description, g.Hint),
						}
						emitEvent(opts.EventWriter, gapCP, "gap.detected")
					}
				}
				if cpLower == "qa-verify" {
					emitEvent(opts.EventWriter, cp, "qa.failed")
				} else {
					emitEvent(opts.EventWriter, cp, "ship.failed")
				}
			}
			return res
		}
		// P2: emit an OK checkpoint span for OTEL-compatible tracing.
		_ = telemetry.EmitCheckpointSpan(root, pipeTraceID, pipeSpanID, strings.ToLower(cp.Name), "OK", 0)

		// Run self-debate for this checkpoint when DebateOpts is set.
		if opts.DebateOpts != nil {
			dOpts := *opts.DebateOpts
			dOpts.Deliverable = strings.ToLower(cp.Name)
			if dOpts.Feature == "" {
				dOpts.Feature = opts.Description
			}
			cp.Debate = SelfDebate(dOpts)
		}

		// Apply gate between checkpoints (not after the last one).
		if opts.Gate != nil && i < total-1 {
			approved := opts.Gate(i, total, cp)
			b := approved
			cp.Approved = &b
			res.Checkpoints = append(res.Checkpoints, cp)
			// G-004: emit event (non-final gated checkpoint).
			if opts.EventWriter != nil {
				emitEvent(opts.EventWriter, cp, "")
			}
			if !approved {
				res.Ready = false
				res.Message = fmt.Sprintf(
					"pipeline stopped after checkpoint %d (%s): reviewer declined to continue",
					i+1, cp.Name,
				)
				// G-004: emit ship.failed on rejection.
				if opts.EventWriter != nil {
					emitEvent(opts.EventWriter, cp, "ship.failed")
				}
				return res
			}
		} else {
			res.Checkpoints = append(res.Checkpoints, cp)
			// G-004: emit event. Only the "ship"/"verify" checkpoint gets ship.passed/failed;
			// other checkpoints always use their natural event name.
			// TG-40: emit gap.detected events before the final ship event.
			if opts.EventWriter != nil {
				cpLower := strings.ToLower(cp.Name)
				switch {
				case cpLower == "ship" || cpLower == "verify":
					// Emit one gap.detected event per gap (before the terminal event).
					if cp.GapAudit != nil {
						for _, g := range cp.GapAudit.Gaps {
							gapCP := Checkpoint{
								Name:   cp.Name,
								Status: g.Severity,
								Detail: fmt.Sprintf("[%s] %s — hint: %s", g.Type, g.Description, g.Hint),
							}
							emitEvent(opts.EventWriter, gapCP, "gap.detected")
						}
					}
					if cp.Status == "fail" {
						emitEvent(opts.EventWriter, cp, "ship.failed")
					} else {
						emitEvent(opts.EventWriter, cp, "ship.passed")
					}
				case cpLower == "qa-verify":
					// Emit gap.detected events before the terminal qa event.
					if cp.GapAudit != nil {
						for _, g := range cp.GapAudit.Gaps {
							gapCP := Checkpoint{
								Name:   cp.Name,
								Status: g.Severity,
								Detail: fmt.Sprintf("[%s] %s — hint: %s", g.Type, g.Description, g.Hint),
							}
							emitEvent(opts.EventWriter, gapCP, "gap.detected")
						}
					}
					if cp.Status == "fail" {
						emitEvent(opts.EventWriter, cp, "qa.failed")
					} else {
						emitEvent(opts.EventWriter, cp, "qa.passed")
					}
				default:
					emitEvent(opts.EventWriter, cp, "")
				}
			}
		}
	}

	res.Ready = true

	// ── Post-pipeline hooks (learning extraction, review routing) ────────────
	if len(hooks) > 0 {
		hookCtx := HookContext{
			Phase:         PhasePostPipeline,
			Root:          root,
			Description:   opts.Description,
			SpecName:      opts.SpecName,
			Pipe:          pipe,
			StrictTesting: hookCfg.StrictTesting,
		}
		runHooks(PhasePostPipeline, hookCtx, hooks, hookCfg) // post-pipeline failures are advisory only
	}

	// ── Learning loop: extract patterns from this successful run ─────────────
	// Writes JSONL + KB-formatted markdown to forge-knowledge when enabled.
	if opts.EnableLearning && pipe != nil {
		extractAndLearnFromFeature(root, opts.Description, res, pipe)
	}

	return res
}

func renderText(cmd *cobra.Command, r *ShipResult) {
	w := cmd.OutOrStdout()

	// ── Single-checkpoint run (forge ship spec|arch|test|...) ─────────────
	// Only use the simplified single-checkpoint view when there are no
	// Yolo/gate/debate fields that require the full pipeline layout.
	isSingle := len(r.Checkpoints) == 1 && !r.Yolo
	if isSingle {
		for _, cp := range r.Checkpoints {
			if cp.Approved != nil || cp.Debate != nil {
				isSingle = false
				break
			}
		}
	}
	if isSingle {
		cp := r.Checkpoints[0]
		marker := "○"
		switch cp.Status {
		case "ok":
			marker = "✓"
		case "fail":
			marker = "✗"
		case "warning":
			marker = "△"
		}
		fmt.Fprintf(w, "%s %s — %s\n", marker, cp.Name, cp.Detail)
		if cp.Status == "fail" {
			fmt.Fprintln(w, "\nFix the issue above, then re-run this checkpoint.")
		} else if cp.Status == "ok" {
			order := []string{"spec", "arch", "test", "breakdown", "code", "ship", "qa-verify"}
			for i, n := range order {
				if strings.EqualFold(cp.Name, n) && i+1 < len(order) {
					fmt.Fprintf(w, "\nnext: forge ship %s \"<description>\"\n", order[i+1])
					fmt.Fprintf(w, "  — or run all remaining: forge ship \"<description>\"\n")
					break
				}
			}
		}
		return
	}

	// ── Full pipeline header ───────────────────────────────────────────────
	mode := ""
	if r.DryRun {
		mode = " --dry-run"
	}
	if r.Yolo {
		mode += " [YOLO — approval gates disabled]"
	}
	fmt.Fprintf(w, "forge ship%s\n", mode)
	if r.Message != "" {
		fmt.Fprintf(w, "%s\n", r.Message)
	}
	fmt.Fprintln(w, "\n7-checkpoint pipeline:")
	for i, cp := range r.Checkpoints {
		marker := "\u25cb " // ○ default (unknown status)
		switch cp.Status {
		case "ok":
			marker = "✓ "
		case "fail":
			marker = "✗ "
		case "warning":
			marker = "\u25b3 " // △ warning
		}
		approval := ""
		if cp.Approved != nil {
			if *cp.Approved {
				approval = " [approved]"
			} else {
				approval = " [rejected]"
			}
		}
		fmt.Fprintf(w, "  [%d/%d] %s%s — %s%s\n", i+1, len(r.Checkpoints), marker, cp.Name, cp.Detail, approval)
		if d := cp.Debate; d != nil {
			consensusIcon := "✓"
			if !d.Consensus {
				consensusIcon = "✗"
			}
			fmt.Fprintf(w, "      \u2726 self-debate [%d roles \u00b7 %d rounds \u00b7 consensus %s \u00b7 %d improvement(s)]\n",
				len(d.Roles), len(d.Rounds), consensusIcon, len(d.Improvements))
			for j, imp := range d.Improvements {
				if j >= 3 {
					fmt.Fprintf(w, "        ... and %d more (--json for full debate output)\n",
						len(d.Improvements)-3)
					break
				}
				fmt.Fprintf(w, "        \u2022 %s\n", imp) // •
			}
		}
	}
	if r.Ready {
		fmt.Fprintln(w, "\nship pipeline ready (all checkpoints validated).")
		if r.FeatureBranch != "" {
			fmt.Fprintf(w, "\nfeature branch: %s\n", r.FeatureBranch)
			fmt.Fprintln(w, "next steps:")
			fmt.Fprintf(w, "  git push origin %s\n", r.FeatureBranch)
			fmt.Fprintf(w, "  gh pr create --base main --head %s --title %q\n",
				r.FeatureBranch, r.Message)
		}
	} else {
		fmt.Fprintln(w, "\nship blocked by checkpoint failure(s) or reviewer rejection.")
	}
}
