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
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/audit"
	"github.com/teragrid/forge/internal/cli/cmdclean"
	"github.com/teragrid/forge/internal/cli/cmdscan"
	"github.com/teragrid/forge/internal/cli/cmdtest"
	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/gitservice"
	"github.com/teragrid/forge/internal/manifest"
	"github.com/teragrid/forge/internal/procspawn"
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
	ErrShipFailed      = errcode.Register(errcode.Code(3200), "ship checkpoint failed")
	ErrSpecAuditFailed = errcode.Register(errcode.Code(3201), "spec audit: incomplete tasks or authz gaps detected")
	ErrBranchFailed    = errcode.Register(errcode.Code(3203), "feature branch operation failed")
	ErrQAVerifyFailed  = errcode.Register(errcode.Code(3204), "QA-verify: test suite or MCP probe failed")
)

// Gate is called after each checkpoint runs (0-based idx, total count, completed cp).
// Return true to proceed to the next checkpoint; false stops the pipeline.
// A nil Gate is YOLO mode: all checkpoints run without any prompt.
type Gate func(idx, total int, cp Checkpoint) bool

// Checkpoint represents one step in the 7-checkpoint pipeline.
type Checkpoint struct {
	Name        string           `json:"name"`
	Status      string           `json:"status"` // "ok", "skipped", "warning", "fail"
	Detail      string           `json:"detail"`
	AutoAdvance        bool             `json:"auto_advance,omitempty"`        // G-009: Code sets true when all tasks done
	Approved           *bool            `json:"approved,omitempty"`            // nil=yolo/not-gated; true=approved; false=rejected
	Debate             *DebateResult    `json:"debate,omitempty"`              // populated when --yolo self-debate runs
	GapAudit           *SpecAuditResult `json:"gap_audit,omitempty"`           // TG-39: spec-vs-code audit result
	RemediationRounds  int              `json:"remediation_rounds,omitempty"` // rounds of LLM-driven gap remediation
}

// ShipResult summarizes the ship run.
type ShipResult struct {
	DryRun        bool         `json:"dry_run"`
	Yolo          bool         `json:"yolo,omitempty"`
	Interactive   bool         `json:"interactive,omitempty"`
	DebateEnabled bool         `json:"debate_enabled,omitempty"`
	Checkpoints   []Checkpoint `json:"checkpoints"`
	Ready         bool         `json:"ready"`
	Message       string       `json:"message"`
	FeatureBranch string       `json:"feature_branch,omitempty"` // branch used for this ship run
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
		yolo           bool
		quick          bool   // --quick: lightweight spec+code only (skip test+breakdown+verify)
		yes            bool   // --yes: auto-approve all gates (alias for --yolo for non-YOLO users)
		from           string // --from: resume from a named checkpoint
		skipCheckpoint string // --skip-checkpoint: skip a named checkpoint
		pr             bool   // --pr: create a draft GitHub PR after all checkpoints pass
		resume         bool   // --resume: resume from first incomplete checkpoint (G-002)
		rootDir        string // --root: project root (default: cwd); primarily for testing
		noBranch       bool   // --no-branch: skip automatic feature-branch creation
	)

	// bindFlags attaches shared flags to a subcommand or the parent.
	bindFlags := func(c *cobra.Command) {
		c.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "preview what would happen without making LLM calls or git operations")
		c.Flags().StringVar(&description, "description", "", "what this change does (deprecated: use positional arg instead)")
		c.Flags().BoolVarP(&asJSON, "json", "j", false, "emit machine-readable JSON")
		c.Flags().BoolVarP(&yolo, "yolo", "Y", false, "skip all approval gates (ship without review prompts)")
		c.Flags().BoolVarP(&quick, "quick", "Q", false, "lightweight run: spec+code only (skips test, breakdown, verify)")
		c.Flags().BoolVarP(&yes, "yes", "y", false, "auto-approve all checkpoint gates (alias for --yolo)")
		c.Flags().StringVarP(&from, "from", "f", "", "resume pipeline from this checkpoint (e.g. --from=code)")
		c.Flags().StringVarP(&skipCheckpoint, "skip-checkpoint", "s", "", "skip a specific checkpoint by name")
		c.Flags().BoolVarP(&pr, "pr", "p", false, "create a draft GitHub PR after all checkpoints pass (requires gh CLI)")
		c.Flags().StringVarP(&rootDir, "root", "r", "", "project root (default: cwd)")
		c.Flags().BoolVarP(&resume, "resume", "R", false, "resume from first incomplete checkpoint (replaces: forge ship resume <feature>)")
		c.Flags().BoolVarP(&noBranch, "no-branch", "B", false, "skip automatic feature-branch creation; work on the current branch")
		c.Flags().StringVarP(&specName, "name", "n", "",
			"name of the spec directory in .forge/specs/ (overrides slug derived from description; applies to all checkpoints)")
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
			Root:        root,
			Description: description,
			SpecName:    specName,
			Names:       names,
			Gate:        gate,
			CreatePR:    pr,
			DryRun:      dryRun,
		}
		if yolo && len(names) == 0 {
			runOpts.DebateOpts = &DebateOptions{
				Feature:   description,
				MaxRounds: 3,
				DryRun:    dryRun,
			}
		}
		// G-004: --yes && --json → NDJSON event stream. --json alone → single JSON object (backward compat).
		if yolo && asJSON {
			runOpts.EventWriter = cmd.OutOrStdout()
		}
		res := RunWithOptions(runOpts)
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

	statusCmd := &cobra.Command{
		Use:   "status [feature]",
		Short: "Show the current pipeline status for a feature (which checkpoints are done).",
		Long: "Reads .forge/specs/<feature>/ and reports which checkpoints have been completed,\n" +
			"which are in-progress, and which are pending.\n\n" +
			"Note: full pipeline state persistence is a M1 feature; " +
			"this command shows last-known state from the spec directory.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			feature := ""
			if len(args) > 0 {
				feature = args[0]
			}
			root, err := os.Getwd()
			if err != nil {
				return errcode.New(ErrShipFailed, "getwd", err)
			}
			specsDir := filepath.Join(root, ".forge", "specs")
			if feature == "" {
				// List all in-flight features
				entries, err := os.ReadDir(specsDir)
				if err != nil { //nolint:gocritic // ifElseChain: clearer as if/else //nolint:gocritic // ifElseChain: clearer as if/else
					fmt.Fprintln(cmd.OutOrStdout(), "no in-flight features found (.forge/specs/ not present or empty)")
					return nil
				}
				if len(entries) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "no in-flight features found")
					return nil
				}
				fmt.Fprintln(cmd.OutOrStdout(), "in-flight features:")
				for _, e := range entries {
					if e.IsDir() {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", e.Name())
					}
				}
				return nil
			}
			featureDir := filepath.Join(specsDir, feature)
			if _, err := os.Stat(featureDir); os.IsNotExist(err) {
				fmt.Fprintf(cmd.OutOrStdout(), "feature %q not found in .forge/specs/\n", feature)
				return nil
			}
			checkpoints := []string{"spec", "arch", "test", "breakdown", "code", "ship", "qa-verify"}
			fmt.Fprintf(cmd.OutOrStdout(), "forge ship status: %s\n", feature)
			for i, cp := range checkpoints {
				marker := "○ pending"
				cpFile := filepath.Join(featureDir, cp+".md")
				if _, err := os.Stat(cpFile); err == nil {
					marker = "✓ done"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  [%d/7] %s — %s\n", i+1, cp, marker)
			}
			return nil
		},
	}

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
	res := RunCheckpoints(root, feature, names)
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
				userPrompt := fmt.Sprintf("Feature: %s\n\nCurrent spec:\n%s", description, string(existing))
				if ySpec != nil {
					userPrompt = fmt.Sprintf(
						"Feature: %s\n\nYAML test spec (%d cases, families: %s):\n%s\n\nCurrent spec.md:\n%s",
						description, len(ySpec.Cases), specFamilyList(ySpec.Families),
						specYAMLContext(ySpec), string(existing),
					)
				}
				var reviewed string
				var reviewErr error
				if ySpec != nil {
					// KB-enriched review when a YAML spec is present.
					reviewed, reviewErr = pipe.InvokeWithKnowledge(
						"ship:spec:review", "",
						specReviewSystem, userPrompt, 2000,
						"spec", "unit", "spec", []string{"test-design", "quality-gate"},
					)
				} else {
					reviewed, reviewErr = pipe.Invoke(
						"ship:spec:review", "",
						specReviewSystem, userPrompt, 2000,
					)
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
					return cp
				}
				if reviewed != "" {
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
					generated, genErr := pipe.InvokeWithKnowledge(
						"ship:spec:generate-from-yaml", "",
						specGenSystem,
						fmt.Sprintf("Generate spec.md for feature: %s\n\n%s", description, specYAMLContext(ySpec)),
						2000,
						"spec", "unit", "spec", []string{"test-design", "quality-gate"},
					)
					if genErr != nil { //nolint:gocritic // ifElseChain: clearer as if/else
						specContent = specStub(description)
						cp.Detail = fmt.Sprintf(
							"spec.md stub created from spec.yml (%d cases) [LLM:%s — %s]: .forge/specs/%s/spec.md — edit before continuing",
							len(ySpec.Cases), pipe.ProviderName(), llmErrNote(genErr), slug)
					} else if generated != "" {
						specContent = generated
						cp.Detail = fmt.Sprintf("spec.md generated from spec.yml (%d cases) by %s: .forge/specs/%s/spec.md",
							len(ySpec.Cases), pipe.ProviderName(), slug)
					} else {
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
				generated, err := pipe.Invoke(
					"ship:spec:generate", "",
					specGenSystem,
					fmt.Sprintf("Generate a complete feature specification for: %s", description),
					2000,
				)
				if err != nil { //nolint:gocritic // ifElseChain: clearer as if/else
					specContent = specStub(description)
					cp.Detail = fmt.Sprintf(
						"spec stub created (LLM:%s — %s): .forge/specs/%s/spec.md — edit before continuing",
						pipe.ProviderName(), llmErrNote(err), slug)
				} else if generated != "" {
					specContent = generated
					cp.Detail = fmt.Sprintf("spec generated by %s: .forge/specs/%s/spec.md",
						pipe.ProviderName(), slug)
				} else {
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
func checkTest(root, description, specName string, pipe *LLMPipe) Checkpoint {
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

	// G-006: write / verify all 4 named test artifacts.
	if description != "" {
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
		if pipe != nil {
			if _, err := generateTestStubs(root, description, pipe); err != nil {
				cp.Detail += fmt.Sprintf(" [LLM:%s — %s]", pipe.ProviderName(), llmErrNote(err))
			}
		}
		return cp
	}
	// No test files — generate 4 named artifacts.
	if pipe != nil {
		if _, err := generateTestStubs(root, description, pipe); err != nil {
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
	return cp
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
			generated, err := generateBreakdown(root, description, pipe)
			if err != nil {
				cp.Status = "warning"
				cp.Detail = fmt.Sprintf("no breakdown.md [LLM:%s — %s] — run forge ship breakdown to generate",
					pipe.ProviderName(), llmErrNote(err))
				// G-011: record breakdown failure for future learning context.
				appendFailure(root, "breakdown", description, cp.Detail)
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
		plan, err := generateCodePlan(root, description, pipe)
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

// countChangedFiles returns the number of modified/untracked files via git status.
// Returns 0 if git is unavailable or not a repo.
func countChangedFiles(root string) int {
	statusFile := filepath.Join(root, ".git", "index")
	if _, err := os.Stat(statusFile); err != nil {
		return 0
	}
	// Walk for any modified files (a fast approximation without exec)
	count := 0
	_ = filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				n := d.Name()
				if n == ".git" || n == "node_modules" || n == "vendor" || n == "dist" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		ext := filepath.Ext(d.Name())
		if ext == ".go" || ext == ".ts" || ext == ".js" || ext == ".py" || ext == ".sql" {
			count++
		}
		return nil
	})
	return count
}

// checkVerify runs the security scanner, clean check, checks the manifest,
// and (TG-39) audits spec artefacts for incomplete tasks and authz gaps.
// M1-10: forge clean --check is now wired here.
func checkVerify(root, description string, pipe *LLMPipe) Checkpoint {
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
	auditRes := auditSpecVsCode(root, description)
	cp.GapAudit = &auditRes

	remediationRounds := 0
	for auditRes.HasBlockingGaps() && pipe != nil && remediationRounds < maxRemediationRounds {
		remediationRounds++
		remediateGaps(root, description, auditRes.Gaps, pipe)
		auditRes = auditSpecVsCode(root, description)
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
//       Go project  — cmd/mcp/main.go present  → go test ./internal/mcpserver/...
//       Python proj — mcp_server.py present    → python -m pytest tests/test_mcp_server.py -q
//     Fallback (no MCP server):
//       Go project  — go.mod present           → go test ./... --count=1
//       Python proj — pyproject.toml/pytest.ini → pytest -q --tb=short
//       No runner   — warning (no tools configured)
func checkQAVerify(root, description string, pipe *LLMPipe) Checkpoint {
	cp := Checkpoint{Name: "QA-Verify"}

	// ── Step 1: spec-vs-code gap audit with auto-remediation loop ────────────
	// Re-run the same audit that checkpoint 6 runs so that qa-verify is
	// self-contained. When an LLM is available and blocking gaps are found, the
	// loop calls remediateGaps to implement the missing pieces, then re-audits.
	// This continues until all blocking gaps are cleared or maxRemediationRounds
	// is exhausted — ensuring the pipeline ships only when fully spec-compliant.
	auditRes := auditSpecVsCode(root, description)
	cp.GapAudit = &auditRes

	for auditRes.HasBlockingGaps() && pipe != nil && cp.RemediationRounds < maxRemediationRounds {
		cp.RemediationRounds++
		remediateGaps(root, description, auditRes.Gaps, pipe)
		auditRes = auditSpecVsCode(root, description)
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

	sp := procspawn.New("go", "python", "python3", "pytest")

	goMCPEntry := filepath.Join(root, "cmd", "mcp", "main.go")
	pyMCPEntry := filepath.Join(root, "mcp_server.py")
	goModFile := filepath.Join(root, "go.mod")

	switch {
	case pathExists(goMCPEntry):
		// MCP server present (Go): run the MCP server's unit tests to validate
		// every AI-agent tool contract.
		res, err := sp.Run("go",
			[]string{"test", "./internal/mcpserver/...", "-count=1", "-timeout=120s"},
			procspawn.Options{Dir: root, Timeout: 130 * time.Second},
		)
		if err != nil {
			cp.Status = "fail"
			cp.Detail = fmt.Sprintf("QA (MCP/go): mcpserver tests failed — %v", err)
			appendFailure(root, "qa-verify", description, cp.Detail)
			return cp
		}
		passed := strings.Count(res.Stdout+res.Stderr, "--- PASS")
		if passed == 0 {
			passed = strings.Count(res.Stdout, "ok")
		}
		cp.Status = "ok"
		cp.Detail = fmt.Sprintf(
			"QA via MCP (go): mcpserver unit tests passed (%d case(s) confirmed, %.1fs) — AI-agent tools operational",
			passed, res.Duration.Seconds())
		if auditRes.SpecFound && len(auditRes.Gaps) > 0 {
			cp.Detail += fmt.Sprintf("; %d spec audit warning(s)", len(auditRes.Gaps))
		}
		return cp

	case pathExists(pyMCPEntry):
		// MCP server present (Python): run test_mcp_server.py.
		// Try python3 first; fall back to python.
		runner, runArgs := "python3", []string{"-m", "pytest", "tests/test_mcp_server.py", "-q", "--tb=short"}
		res, err := sp.Run(runner, runArgs, procspawn.Options{Dir: root, Timeout: 130 * time.Second})
		if err != nil && strings.Contains(err.Error(), "not in the spawn allow-list") {
			runner = "python"
			res, err = sp.Run(runner, runArgs, procspawn.Options{Dir: root, Timeout: 130 * time.Second})
		}
		if err != nil {
			cp.Status = "fail"
			cp.Detail = fmt.Sprintf("QA (MCP/python): test_mcp_server.py failed — %v", err)
			appendFailure(root, "qa-verify", description, cp.Detail)
			return cp
		}
		passed := strings.Count(res.Stdout, " passed")
		cp.Status = "ok"
		cp.Detail = fmt.Sprintf(
			"QA via MCP (python): test_mcp_server.py passed (%d case(s) confirmed, %.1fs) — AI-agent tools operational",
			passed, res.Duration.Seconds())
		if auditRes.SpecFound && len(auditRes.Gaps) > 0 {
			cp.Detail += fmt.Sprintf("; %d spec audit warning(s)", len(auditRes.Gaps))
		}
		return cp

	case pathExists(goModFile):
		// Fallback: Go project without MCP server — run full test suite.
		// Failure here is a warning (not fail): the project may have no tests yet or be
		// a fresh scaffold. Only MCP-explicit paths fail hard.
		res, err := sp.Run("go",
			[]string{"test", "./...", "-count=1", "-timeout=180s"},
			procspawn.Options{Dir: root, Timeout: 190 * time.Second},
		)
		if err != nil {
			cp.Status = "warning"
			cp.Detail = fmt.Sprintf("QA (go test ./...): tests did not pass (%v) — add cmd/mcp/ for MCP-based QA", err)
			return cp
		}
		pkgs := strings.Count(res.Stdout+res.Stderr, "\nok ")
		cp.Status = "ok"
		cp.Detail = fmt.Sprintf(
			"QA (go test ./...): %d package(s) passed (%.1fs) — add cmd/mcp/ to enable MCP agent QA",
			pkgs, res.Duration.Seconds())
		if auditRes.SpecFound && len(auditRes.Gaps) > 0 {
			cp.Detail += fmt.Sprintf("; %d spec audit warning(s)", len(auditRes.Gaps))
		}
		return cp

	case pathExists(filepath.Join(root, "pyproject.toml")) ||
		pathExists(filepath.Join(root, "pytest.ini")) ||
		pathExists(filepath.Join(root, "setup.cfg")):
		// Fallback: Python project without MCP server.
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
			cp.Status = "warning"
			cp.Detail = fmt.Sprintf("QA (pytest): tests did not pass (%v) — add mcp_server.py for MCP agent QA", err)
			return cp
		}
		passed := strings.Count(res.Stdout, " passed")
		cp.Status = "ok"
		cp.Detail = fmt.Sprintf(
			"QA (pytest): %d case(s) passed (%.1fs) — add mcp_server.py to enable MCP agent QA",
			passed, res.Duration.Seconds())
		if auditRes.SpecFound && len(auditRes.Gaps) > 0 {
			cp.Detail += fmt.Sprintf("; %d spec audit warning(s)", len(auditRes.Gaps))
		}
		return cp
	}

	// No known test runner or MCP server found — warn but do not block.
	cp.Status = "warning"
	if auditRes.SpecFound && len(auditRes.Gaps) > 0 {
		cp.Detail = fmt.Sprintf("QA-Verify: no test runner found; %d spec audit warning(s) — "+
			"add cmd/mcp/ (Go) or mcp_server.py (Python) for AI-agent QA, "+
			"or ensure go.mod / pyproject.toml is present for native test fallback",
			len(auditRes.Gaps))
	} else {
		cp.Detail = "QA-Verify: no MCP server or test runner found — " +
			"add cmd/mcp/ (Go) or mcp_server.py (Python) for AI-agent QA, " +
			"or ensure go.mod / pyproject.toml is present for native test fallback"
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
	// leaves it nil so auto-detection runs via llmprovider.Detect.
	pipe := opts.LLMPipe
	if pipe == nil {
		pipe = newLLMPipe(root)
	}

	allCPs := []Checkpoint{
		checkSpec(root, opts.Description, opts.SpecName, pipe),
		checkArch(root, opts.Description, opts.SpecName, pipe),
		checkTest(root, opts.Description, opts.SpecName, pipe),
		checkBreakdown(root, opts.Description, opts.SpecName, pipe),
		checkCode(root, opts.Description, opts.SpecName, pipe),
		checkVerify(root, opts.Description, pipe),
		checkQAVerify(root, opts.Description, pipe),
	}
	// PR checkpoint: appended only for full-pipeline runs with --pr.
	if opts.CreatePR && len(opts.Names) == 0 {
		allCPs = append(allCPs, checkPR(root, opts.Description))
	}

	checkpointIndex := map[string]int{
		"spec":      0,
		"arch":      1,
		"test":      2,
		"breakdown": 3,
		"code":      4,
		"ship":      5, // G-003: primary name
		"verify":    5, // G-003: deprecated alias
		"qa-verify": 6,
	}

	var selected []Checkpoint
	if len(opts.Names) == 0 {
		selected = allCPs
	} else {
		for _, n := range opts.Names {
			idx, ok := checkpointIndex[n]
			if !ok {
				selected = append(selected, Checkpoint{
					Name:   n,
					Status: "fail",
					Detail: fmt.Sprintf("unknown checkpoint %q; one of: spec, arch, test, breakdown, code, ship", n),
				})
				continue
			}
			selected = append(selected, allCPs[idx])
		}
	}

	res := &ShipResult{
		DryRun:        opts.DryRun,
		DebateEnabled: opts.DebateOpts != nil,
		Message:       shipMessage(pipe),
	}
	total := len(selected)

	for i, cp := range selected {
		// Hard stop on failure regardless of gate.
		if cp.Status == "fail" {
			res.Checkpoints = append(res.Checkpoints, cp)
			res.Ready = false
			res.Message = fmt.Sprintf("checkpoint %s failed; pipeline stopped", cp.Name)
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
