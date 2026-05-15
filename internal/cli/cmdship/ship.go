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
//	spec        â€“ checkpoint 1: validate / generate the feature spec
//	test        â€“ checkpoint 2: generate failing tests (TDD gate)
//	breakdown   â€“ checkpoint 3: decompose spec into AI-friendly tasks
//	code        â€“ checkpoint 4: generate / iterate code until tests are green
//	verify      â€“ checkpoint 5: hygiene + scan + lint readiness check
//
// Running `forge ship` (no subcommand) runs all five checkpoints in sequence.
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

	"github.com/teragrid/forge/internal/cli/cmdclean"
	"github.com/teragrid/forge/internal/cli/cmdscan"
	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/manifest"
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
	"test":      "tests.generated",
	"breakdown": "tasks.broken-down",
	"code":      "task.completed",
	"ship":      "ship.passed",
	"verify":    "ship.passed", // deprecated alias
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
	ErrShipFailed = errcode.Register(errcode.Code(3200), "ship checkpoint failed")
)

// Gate is called after each checkpoint runs (0-based idx, total count, completed cp).
// Return true to proceed to the next checkpoint; false stops the pipeline.
// A nil Gate is YOLO mode: all checkpoints run without any prompt.
type Gate func(idx, total int, cp Checkpoint) bool

// Checkpoint represents one step in the 5-checkpoint pipeline.
type Checkpoint struct {
	Name     string        `json:"name"`
	Status   string        `json:"status"` // "ok", "skipped", "warning", "fail"
	Detail   string        `json:"detail"`
	Approved *bool         `json:"approved,omitempty"` // nil=yolo/not-gated; true=approved; false=rejected
	Debate   *DebateResult `json:"debate,omitempty"`   // populated when --yolo self-debate runs
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
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb: "ship",
		Summary: "Deploy a change through the 5-checkpoint pipeline " +
			"(spec â†’ test â†’ breakdown â†’ code â†’ verify). " +
			"Each checkpoint requires reviewer approval before the next runs (interactive mode). " +
			"Use --yolo to skip all approval gates. " +
			"Run a single checkpoint with: forge ship spec|test|breakdown|code|verify. " +
			"M1 full impl; MVP is --dry-run preview.",
		Inputs: []string{
			"[subcommand]: spec | test | breakdown | code | verify (optional; runs all when omitted)",
			"--dry-run (validate checkpoints without executing; default in MVP)",
			"--description <msg> (what this change does; required for full pipeline in M1)",
			"--yolo (skip all approval gates â€” activates 6-role self-debate for quality polishing)",
			"--json (machine-readable output; also disables interactive prompts)",
		},
		Outputs: []string{"stdout: checkpoint status (text or JSON)"},
		SideEffects: []string{
			"--dry-run has no side effects; full workflow (M1) will commit, tag, and deploy",
		},
		GatesTouched: []string{"Â§16.5.2 ship workflow", "Â§4 5-checkpoint pipeline"},
		ErrorCodes:   []errcode.Code{ErrShipFailed},
	})
}

// makeInteractiveGate returns a Gate that reads y/N from scanner and writes
// prompts to out. It is called between checkpoints (never after the last one).
func makeInteractiveGate(scanner *bufio.Scanner, out io.Writer) Gate {
	return func(idx, total int, cp Checkpoint) bool {
		marker := "â—‹"
		switch cp.Status {
		case "ok":
			marker = "âœ“"
		case "fail":
			marker = "âœ—"
		case "warning":
			marker = "âš "
		}
		fmt.Fprintf(out, "\n  [%d/%d] %s %s â€” %s\n", idx+1, total, marker, cp.Name, cp.Detail)
		fmt.Fprintf(out, "       â†’ Approve and continue to checkpoint %d/%d? [y/N] ", idx+2, total)
		if !scanner.Scan() {
			return true // EOF / closed stdin â†’ auto-approve (non-interactive caller)
		}
		return strings.ToLower(strings.TrimSpace(scanner.Text())) == "y"
	}
}

// New returns the cobra command with checkpoint subcommands.
func New() *cobra.Command {
	var (
		dryRun         bool
		description    string
		asJSON         bool
		yolo           bool
		quick          bool   // --quick: lightweight spec+code only (skip test+breakdown+verify)
		yes            bool   // --yes: auto-approve all gates (alias for --yolo for non-YOLO users)
		from           string // --from: resume from a named checkpoint
		skipCheckpoint string // --skip-checkpoint: skip a named checkpoint
		pr             bool   // --pr: create a draft GitHub PR after all checkpoints pass
		resume         bool   // --resume: resume from first incomplete checkpoint (G-002)
		rootDir        string // --root: project root (default: cwd); primarily for testing
	)

	// bindFlags attaches shared flags to a subcommand or the parent.
	bindFlags := func(c *cobra.Command) {
		c.Flags().BoolVar(&dryRun, "dry-run", true, "validate without executing (default in MVP)")
		c.Flags().StringVar(&description, "description", "", "what this change does")
		c.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
		c.Flags().BoolVar(&yolo, "yolo", false, "skip all approval gates (ship without review prompts)")
		c.Flags().BoolVar(&quick, "quick", false, "lightweight run: spec+code only (skips test, breakdown, verify)")
		c.Flags().BoolVar(&yes, "yes", false, "auto-approve all checkpoint gates (alias for --yolo)")
		c.Flags().StringVar(&from, "from", "", "resume pipeline from this checkpoint (e.g. --from=code)")
		c.Flags().StringVar(&skipCheckpoint, "skip-checkpoint", "", "skip a specific checkpoint by name")
		c.Flags().BoolVar(&pr, "pr", false, "create a draft GitHub PR after all checkpoints pass (requires gh CLI)")
		c.Flags().StringVar(&rootDir, "root", "", "project root (default: cwd)")
		c.Flags().BoolVar(&resume, "resume", false, "resume from first incomplete checkpoint (replaces: forge ship resume <feature>)")
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
			order := []string{"spec", "test", "breakdown", "code", "ship"}
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
					"--from: unknown checkpoint %q; one of: spec, test, breakdown, code, ship", from)
			}
		}

		// --skip-checkpoint: remove a named checkpoint from the run list.
		if skipCheckpoint != "" {
			if len(names) == 0 {
				names = []string{"spec", "test", "breakdown", "code", "ship"}
			}
			filtered := names[:0]
			for _, n := range names {
				if n != skipCheckpoint {
					filtered = append(filtered, n)
				}
			}
			names = filtered
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
			Names:       names,
			Gate:        gate,
			CreatePR:    pr,
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
		Use:   "ship [<feature>] [spec|test|breakdown|code|ship] [flags]",
		Short: "Deploy a change through the 5-checkpoint pipeline.",
		Long: "forge ship walks a change through 5 checkpoints:\n" +
			"  (1) spec      â€” validate or generate the feature spec\n" +
			"  (2) test      â€” generate failing tests (TDD gate)\n" +
			"  (3) breakdown â€” decompose spec into AI-friendly tasks\n" +
			"  (4) code      â€” generate / iterate code until tests pass\n" +
			"  (5) verify    â€” hygiene + scan + lint readiness check\n\n" +
			"Run a single checkpoint with: forge ship spec|test|breakdown|code|verify\n" +
			"Run all five checkpoints with: forge ship (no subcommand)\n\n" +
			"The --dry-run flag (default in MVP) validates checkpoints without executing.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			Use:   name,
			Short: short,
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return runCheckpoint(cmd, []string{name})
			},
		}
		bindFlags(c)
		return c
	}

	// G-003: checkpoint 5 renamed "ship"; "verify" kept as deprecated alias.
	verifyDeprecated := makeCheckpointCmd("verify",
		"[deprecated] Checkpoint 5: use 'forge ship ship' instead.")
	verifyDeprecated.Deprecated = "use 'forge ship ship' instead"
	cmd.AddCommand(
		makeCheckpointCmd("spec",
			"Checkpoint 1: validate or generate the feature spec"),
		makeCheckpointCmd("test",
			"Checkpoint 2: generate failing tests before any code (TDD gate)"),
		makeCheckpointCmd("breakdown",
			"Checkpoint 3: decompose the spec into AI-friendly task list"),
		makeCheckpointCmd("code",
			"Checkpoint 4: generate / iterate code until tests pass"),
		makeCheckpointCmd("ship",
			"Checkpoint 5: hygiene + scan + lint ship-readiness check"),
		verifyDeprecated,
	)

	// â”€â”€ Status + resume subcommands (spec Â§4 ship sub-verbs) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

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
			checkpoints := []string{"spec", "test", "breakdown", "code", "ship"}
			fmt.Fprintf(cmd.OutOrStdout(), "forge ship status: %s\n", feature)
			for i, cp := range checkpoints {
				marker := "â—‹ pending"
				cpFile := filepath.Join(featureDir, cp+".md")
				if _, err := os.Stat(cpFile); err == nil {
					marker = "âœ“ done"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  [%d/5] %s â€” %s\n", i+1, cp, marker)
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
			checkpoints := []string{"spec", "test", "breakdown", "code", "ship"}
			resumeFrom := ""
			for _, cp := range checkpoints {
				cpFile := filepath.Join(specsDir, cp+".md")
				if _, err := os.Stat(cpFile); os.IsNotExist(err) {
					resumeFrom = cp
					break
				}
			}
			if resumeFrom == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "forge ship resume: %s â€” all checkpoints complete\n", feature)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "forge ship resume: %s â€” resuming from checkpoint %q\n", feature, resumeFrom)
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
	checkpoints := []string{"spec", "test", "breakdown", "code", "ship"}
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

// Run executes the full 5-checkpoint dry-run validation (backward-compat entry point).
func Run(root, description string) *ShipResult {
	return RunCheckpoints(root, description, nil)
}

// â”€â”€ Per-checkpoint evaluators â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// checkSpec validates or generates the feature spec.
// With an LLMPipe: if the spec already exists it is reviewed and enhanced;
// if it does not exist it is generated from the description.
// Without an LLMPipe (no provider configured): a Markdown stub is written.
func checkSpec(root, description string, pipe *LLMPipe) Checkpoint {
	cp := Checkpoint{Name: "Spec"}
	specsDir := filepath.Join(root, ".forge", "specs")
	if description != "" {
		slug := slugify(description)
		specFile := filepath.Join(specsDir, slug, "spec.md")
		if _, err := os.Stat(specFile); err == nil {
			// Spec exists â€” attempt LLM review.
			if pipe != nil {
				existing, _ := os.ReadFile(specFile)
				reviewed, err := pipe.Invoke(
					"ship:spec:review", "",
					"You are a senior software architect reviewing a feature specification. "+
						"Improve its acceptance criteria (Given/When/Then format), NFRs, and edge cases. "+
						"Return the full improved specification in Markdown.",
					fmt.Sprintf("Feature: %s\n\nCurrent spec:\n%s", description, string(existing)),
					2000,
				)
				if err != nil { //nolint:gocritic // ifElseChain: clearer as if/else //nolint:gocritic // ifElseChain: clearer as if/else
					cp.Status = "ok"
					cp.Detail = fmt.Sprintf("spec found: .forge/specs/%s/spec.md [LLM:%s â€” %s]",
						slug, pipe.ProviderName(), llmErrNote(err))
					return cp
				}
				if reviewed != "" {
					_ = os.WriteFile(specFile, []byte(reviewed), 0o600)
					cp.Status = "ok"
					cp.Detail = fmt.Sprintf("spec reviewed and enhanced by %s: .forge/specs/%s/spec.md",
						pipe.ProviderName(), slug)
					return cp
				}
			}
			cp.Status = "ok"
			cp.Detail = fmt.Sprintf("spec found: .forge/specs/%s/spec.md", slug)
			return cp
		}
		// Spec does not exist â€” generate via LLM or write a stub.
		if err := os.MkdirAll(filepath.Join(specsDir, slug), 0o755); err == nil {
			specContent := ""
			if pipe != nil {
				generated, err := pipe.Invoke(
					"ship:spec:generate", "",
					"You are a senior software architect. Generate a feature specification in Markdown. "+
						"Include: What, Why, Acceptance Criteria (Given/When/Then format), "+
						"Non-functional requirements, and Out of scope.",
					fmt.Sprintf("Generate a complete feature specification for: %s", description),
					2000,
				)
				if err != nil { //nolint:gocritic // ifElseChain: clearer as if/else
					specContent = specStub(description)
					cp.Detail = fmt.Sprintf(
						"spec stub created (LLM:%s â€” %s): .forge/specs/%s/spec.md â€” edit before continuing",
						pipe.ProviderName(), llmErrNote(err), slug)
				} else if generated != "" {
					specContent = generated
					cp.Detail = fmt.Sprintf("spec generated by %s: .forge/specs/%s/spec.md",
						pipe.ProviderName(), slug)
				} else {
					specContent = specStub(description)
					cp.Detail = fmt.Sprintf("spec stub created: .forge/specs/%s/spec.md â€” edit before continuing", slug)
				}
			} else {
				specContent = specStub(description)
				cp.Detail = fmt.Sprintf(
					"spec stub created: .forge/specs/%s/spec.md â€” edit before continuing "+
						"(set ANTHROPIC_API_KEY to auto-generate via LLM)",
					slug)
			}
			if err := os.WriteFile(specFile, []byte(specContent), 0o600); err == nil {
				cp.Status = "ok"
				return cp
			}
		}
		// .forge/ not writable â€” still ok for dry-run; record the description.
		cp.Status = "ok"
		cp.Detail = fmt.Sprintf("description: %q (write .forge/specs/ to persist)", description)
		return cp
	}
	// No description â€” look for any existing spec.
	entries, err := os.ReadDir(specsDir)
	if err == nil && len(entries) > 0 {
		cp.Status = "ok"
		cp.Detail = fmt.Sprintf("%d spec(s) in .forge/specs/; pass --description <feature> to target one", len(entries))
		return cp
	}
	cp.Status = "warning"
	if pipe != nil {
		cp.Detail = fmt.Sprintf("no --description; pass --description <feature> to generate a spec via %s",
			pipe.ProviderName())
	} else {
		cp.Detail = "no --description and no specs in .forge/specs/; pass --description <feature> to generate a spec stub"
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
// M1-03: git timestamp guard â€” test files must predate or match their
// corresponding production files. If any prod file is newer than its test
// file by more than 60 seconds the checkpoint is marked as a failure.
func checkTest(root, description string, pipe *LLMPipe) Checkpoint {
	cp := Checkpoint{Name: "Test"}
	testFiles := findTestFiles(root)

	// M1-03: tests-precede-code timestamp guard.
	if violations := testTimestampGuard(root); len(violations) > 0 {
		cp.Status = "fail"
		cp.Detail = fmt.Sprintf(
			"tests-precede-code violation: %d production file(s) were modified more than 60s before their test file â€” write failing tests first: %s",
			len(violations), strings.Join(violations[:min3(len(violations))], ", "),
		)
		return cp
	}

	if len(testFiles) > 0 {
		cp.Status = "ok"
		cp.Detail = fmt.Sprintf("%d test file(s) found", len(testFiles))
		if pipe != nil {
			stub, err := generateTestStubs(root, description, pipe)
			if err != nil {
				cp.Detail += fmt.Sprintf(" [LLM:%s â€” %s]", pipe.ProviderName(), llmErrNote(err))
			} else if stub != "" {
				cp.Detail = fmt.Sprintf("%d test file(s) found; test stubs written by %s to .forge/specs/%s/test-stubs.md",
					len(testFiles), pipe.ProviderName(), slugify(description))
			}
		}
		return cp
	}
	// No test files â€” attempt LLM generation.
	if pipe != nil {
		stub, err := generateTestStubs(root, description, pipe)
		if err != nil {
			cp.Status = "warning"
			cp.Detail = fmt.Sprintf("no test files found [LLM:%s â€” %s] â€” write failing tests before forge ship code",
				pipe.ProviderName(), llmErrNote(err))
			return cp
		}
		if stub != "" {
			cp.Status = "ok"
			cp.Detail = fmt.Sprintf("test stubs generated by %s (see .forge/specs/%s/test-stubs.md) â€” complete before forge ship code",
				pipe.ProviderName(), slugify(description))
			return cp
		}
	}
	cp.Status = "warning"
	cp.Detail = "no test files found â€” write failing tests before running forge ship code"
	if pipe == nil {
		cp.Detail += " (set ANTHROPIC_API_KEY to auto-generate stubs)"
	}
	return cp
}

// testTimestampGuard returns the names of production Go files that are newer
// than their corresponding _test.go files by more than 60 seconds.
// Only files in the working tree (not .git/, vendor/, node_modules/) are checked.
func testTimestampGuard(root string) []string {
	const maxDrift = 60 * 1000000000 // 60 seconds in nanoseconds
	var violations []string
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
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		// Find corresponding test file.
		testPath := strings.TrimSuffix(p, ".go") + "_test.go"
		prodInfo, err := d.Info()
		if err != nil {
			return nil
		}
		testInfo, err := os.Stat(testPath)
		if err != nil {
			// No test file at all â€” not a timestamp violation (handled elsewhere).
			return nil
		}
		diff := prodInfo.ModTime().UnixNano() - testInfo.ModTime().UnixNano()
		if diff > maxDrift {
			violations = append(violations, strings.TrimPrefix(p, root+string(os.PathSeparator)))
		}
		return nil
	})
	return violations
}

// min3 returns min(n, 3) â€” used to cap violation list display.
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
func checkBreakdown(root, description string, pipe *LLMPipe) Checkpoint {
	cp := Checkpoint{Name: "Breakdown"}
	if description != "" {
		slug := slugify(description)
		breakdownFile := filepath.Join(root, ".forge", "specs", slug, "breakdown.md")
		if _, err := os.Stat(breakdownFile); err == nil {
			cp.Status = "ok"
			cp.Detail = fmt.Sprintf("breakdown found: .forge/specs/%s/breakdown.md", slug)
			return cp
		}
		// Breakdown does not exist â€” attempt LLM generation.
		if pipe != nil {
			generated, err := generateBreakdown(root, description, pipe)
			if err != nil {
				cp.Status = "warning"
				cp.Detail = fmt.Sprintf("no breakdown.md [LLM:%s â€” %s] â€” run forge ship breakdown to generate",
					pipe.ProviderName(), llmErrNote(err))
				return cp
			}
			if generated != "" {
				cp.Status = "ok"
				cp.Detail = fmt.Sprintf("breakdown generated by %s: .forge/specs/%s/breakdown.md",
					pipe.ProviderName(), slug)
				return cp
			}
		}
	}
	// Structural fallback.
	if _, err := os.Stat(filepath.Join(root, ".forge", "specs")); err == nil {
		cp.Status = "warning"
		cp.Detail = "no breakdown.md found â€” run forge ship breakdown to generate"
		if pipe == nil {
			cp.Detail += " (set ANTHROPIC_API_KEY to auto-generate)"
		}
	} else {
		cp.Status = "warning"
		cp.Detail = "no .forge/specs/ directory â€” run forge ship spec first"
	}
	return cp
}

// checkCode verifies working-tree changes and (when an LLMPipe is available)
// generates a step-by-step code implementation plan from the spec+breakdown.
func checkCode(root, description string, pipe *LLMPipe) Checkpoint {
	cp := Checkpoint{Name: "Code"}
	changedFiles := countChangedFiles(root)

	if pipe != nil {
		plan, err := generateCodePlan(root, description, pipe)
		if err != nil {
			if changedFiles > 0 {
				cp.Status = "ok"
				cp.Detail = fmt.Sprintf("%d modified file(s) [LLM:%s â€” %s]",
					changedFiles, pipe.ProviderName(), llmErrNote(err))
			} else {
				cp.Status = "warning"
				cp.Detail = fmt.Sprintf("no code changes detected [LLM:%s â€” %s]",
					pipe.ProviderName(), llmErrNote(err))
			}
			return cp
		}
		if plan != "" {
			if changedFiles > 0 {
				cp.Status = "ok"
				cp.Detail = fmt.Sprintf("%d modified file(s); code plan written by %s (see .forge/specs/%s/code-plan.md)",
					changedFiles, pipe.ProviderName(), slugify(description))
			} else {
				cp.Status = "ok"
				cp.Detail = fmt.Sprintf("code plan written by %s (see .forge/specs/%s/code-plan.md) â€” implement then rerun",
					pipe.ProviderName(), slugify(description))
			}
			return cp
		}
	}

	// Structural fallback (no LLM or no spec/breakdown context).
	if changedFiles > 0 {
		cp.Status = "ok"
		cp.Detail = fmt.Sprintf("%d modified file(s) detected in working tree", changedFiles)
		return cp
	}
	cp.Status = "warning"
	cp.Detail = "no code changes detected in working tree â€” implement tasks then rerun forge ship code"
	if pipe == nil {
		cp.Detail += " (set ANTHROPIC_API_KEY to auto-generate a code plan)"
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

// checkVerify runs the security scanner, clean check, and checks the manifest.
// This is the only checkpoint with real automated gates in MVP.
// M1-10: forge clean --check is now wired here.
func checkVerify(root string) Checkpoint {
	cp := Checkpoint{Name: "Ship"}

	// Run security scan
	scanRes, err := cmdscan.RunSecurity(root)
	if err != nil {
		cp.Status = "warning"
		cp.Detail = fmt.Sprintf("security scan error: %v", err)
		return cp
	}
	if len(scanRes.Findings) > 0 {
		cp.Status = "fail"
		cp.Detail = fmt.Sprintf("security scan: %d finding(s) â€” fix before shipping (run: forge scan security)", len(scanRes.Findings))
		return cp
	}

	// M1-10: forge clean --check â€” block ship if unmanaged scratch files exist.
	cleanRes, cleanErr := cmdclean.Run(root, false /* check mode */)
	if cleanErr != nil {
		cp.Status = "warning"
		cp.Detail = fmt.Sprintf("clean check error: %v â€” run `forge clean --check` manually", cleanErr)
		return cp
	}
	if len(cleanRes.Candidates) > 0 || len(cleanRes.TrackedSecrets) > 0 {
		detail := fmt.Sprintf("hygiene: %d unmanaged file(s)", len(cleanRes.Candidates))
		if len(cleanRes.TrackedSecrets) > 0 {
			detail += fmt.Sprintf("; %d tracked secret(s)", len(cleanRes.TrackedSecrets))
		}
		cp.Status = "fail"
		cp.Detail = detail + " â€” run `forge clean --apply` to remove, then re-run forge ship verify"
		return cp
	}

	// Check manifest
	mf, _ := manifest.Load(filepath.Join(root, manifest.DefaultPath))
	patternCount := len(mf.Scratch) + len(mf.Managed)
	cp.Status = "ok"
	cp.Detail = fmt.Sprintf("security scan clean; hygiene OK; manifest OK (%d patterns)", patternCount)
	return cp
}

// RunCheckpoints executes the requested checkpoints (nil = all five) in YOLO mode
// (no approval gates, no self-debate). Backward-compatible entry point.
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
		checkSpec(root, opts.Description, pipe),
		checkTest(root, opts.Description, pipe),
		checkBreakdown(root, opts.Description, pipe),
		checkCode(root, opts.Description, pipe),
		checkVerify(root),
	}
	// PR checkpoint: appended only for full-pipeline runs with --pr.
	if opts.CreatePR && len(opts.Names) == 0 {
		allCPs = append(allCPs, checkPR(root, opts.Description))
	}

	checkpointIndex := map[string]int{
		"spec":      0,
		"test":      1,
		"breakdown": 2,
		"code":      3,
		"ship":      4, // G-003: primary name
		"verify":    4, // G-003: deprecated alias
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
					Detail: fmt.Sprintf("unknown checkpoint %q; one of: spec, test, breakdown, code, verify", n),
				})
				continue
			}
			selected = append(selected, allCPs[idx])
		}
	}

	res := &ShipResult{
		DryRun:        true,
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
			// G-004: emit ship.failed event.
			if opts.EventWriter != nil {
				emitEvent(opts.EventWriter, cp, "ship.failed")
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
			if opts.EventWriter != nil {
				cpLower := strings.ToLower(cp.Name)
				if (cpLower == "ship" || cpLower == "verify") && cp.Status == "fail" {
					emitEvent(opts.EventWriter, cp, "ship.failed")
				} else if cpLower == "ship" || cpLower == "verify" {
					emitEvent(opts.EventWriter, cp, "ship.passed")
				} else {
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
	mode := "--dry-run"
	if r.Yolo {
		mode += " [YOLO â€” approval gates disabled]"
	}
	fmt.Fprintf(w, "forge ship %s\n", mode)
	fmt.Fprintf(w, "%s\n", r.Message)
	fmt.Fprintln(w, "\n5-checkpoint pipeline:")
	for i, cp := range r.Checkpoints {
		marker := "âŠ˜ "
		if cp.Status == "ok" {
			marker = "âœ“ "
		} else if cp.Status == "fail" {
			marker = "âœ— "
		}
		approval := ""
		if cp.Approved != nil {
			if *cp.Approved {
				approval = " [approved]"
			} else {
				approval = " [rejected]"
			}
		}
		fmt.Fprintf(w, "  [%d] %s%s â€” %s%s\n", i+1, marker, cp.Name, cp.Detail, approval)
		if d := cp.Debate; d != nil {
			consensusIcon := "âœ“"
			if !d.Consensus {
				consensusIcon = "âœ—"
			}
			fmt.Fprintf(w, "      âœ¦ self-debate [%d roles Â· %d rounds Â· consensus %s Â· %d improvement(s)]\n",
				len(d.Roles), len(d.Rounds), consensusIcon, len(d.Improvements))
			for j, imp := range d.Improvements {
				if j >= 3 {
					fmt.Fprintf(w, "        â‹¯ and %d more (--json for full debate output)\n",
						len(d.Improvements)-3)
					break
				}
				fmt.Fprintf(w, "        â€¢ %s\n", imp)
			}
		}
	}
	if r.Ready {
		fmt.Fprintln(w, "\nship pipeline ready (all checkpoints validated).")
	} else {
		fmt.Fprintln(w, "\nship blocked by checkpoint failure(s) or reviewer rejection.")
	}
}
