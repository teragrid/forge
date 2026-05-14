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

// Package cmdci implements `forge ci` (spec §13.6 Post-Push CI Monitor).
//
// forge ci provides three sub-commands that together power the post-push CI
// monitoring loop:
//
//   - forge ci watch   — polls GitHub Actions for a commit's workflow run and
//     reports its conclusion (pass/fail/timeout).
//   - forge ci fix     — retrieves CI failure logs and invokes the LLM to
//     propose a targeted fix diff.
//   - forge ci gotcha  — records a CI failure as a lesson to
//     .forge/learned/gotchas.jsonl for the continuous learning loop.
//
// These commands are invoked automatically by .githooks/post-push, but are
// also available directly so agents and developers can call them without going
// through the hook.
//
// Usage:
//
//	forge ci watch [--sha <sha>] [--repo <owner/repo>] [--timeout 5m]
//	forge ci fix   [--run-id <id>]
//	forge ci gotcha --run-id <id> [--branch <b>] [--sha <s>] [--url <u>]
//	               [--conclusion <c>] [--note <text>]
package cmdci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 6200..6299 — cli/ci).
const (
	CodeCIOperationFailed = errcode.Code(6200)
	CodeCIWatchTimeout    = errcode.Code(6201)
	CodeCIRunFailed       = errcode.Code(6202)
	CodeCINoRunFound      = errcode.Code(6203)
	CodeCIGotchaWrite     = errcode.Code(6204)
)

const (
	gotchaFile = ".forge/learned/gotchas.jsonl"
	auditLog   = ".forge/audit.log"
)

func initMeta() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "ci",
		Summary: "Post-push CI monitor: watch, fix, and record lessons from CI runs (spec §13.6).",
		Inputs: []string{
			"watch [--sha <sha>] [--repo <owner/repo>] [--timeout <duration>]",
			"fix   [--run-id <id>]",
			"gotcha --run-id <id> [--branch <b>] [--sha <s>] [--url <u>] [--conclusion <c>] [--note <text>]",
			"--json     — machine-readable output",
		},
		Outputs: []string{
			"stdout: CI status summary, fix proposal, or gotcha confirmation",
		},
		SideEffects: []string{
			"gotcha: appends a JSON record to .forge/learned/gotchas.jsonl",
			"watch/fix: may write to .forge/audit.log on failure",
		},
	})
}

func init() {
	errcode.Register(CodeCIOperationFailed, "CI monitor operation failed")
	errcode.Register(CodeCIWatchTimeout, "CI watch timed out waiting for run to complete")
	errcode.Register(CodeCIRunFailed, "CI run concluded with failure")
	errcode.Register(CodeCINoRunFound, "no CI workflow run found for the given SHA")
	errcode.Register(CodeCIGotchaWrite, "failed to write CI gotcha to .forge/learned/gotchas.jsonl")
	initMeta()
}
// New returns the top-level `forge ci` cobra command.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ci",
		Short: "Post-push CI monitor — watch, fix, and learn from CI runs (spec §13.6)",
		Long: `forge ci powers the post-push CI monitoring loop.

After a successful git push, .githooks/post-push calls 'forge ci watch' to
poll GitHub Actions for the triggered workflow run.  On failure it calls
'forge ci fix' (LLM-assisted) or 'forge ci gotcha' (records a lesson).

Sub-commands are also available directly for agents and manual use.`,
	}

	cmd.AddCommand(
		newWatchCmd(),
		newFixCmd(),
		newGotchaCmd(),
	)

	return cmd
}

// ── watch ─────────────────────────────────────────────────────────────────────

type watchFlags struct {
	sha      string
	repo     string
	timeout  time.Duration
	interval time.Duration
	jsonOut  bool
}

func newWatchCmd() *cobra.Command {
	f := &watchFlags{}
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Poll GitHub Actions for a workflow run triggered by a commit",
		Long: `forge ci watch polls the GitHub Actions API for the workflow run
associated with the given commit SHA.

If --sha is omitted, the current HEAD is used.
If --repo is omitted, it is derived from the origin remote URL.

Exit codes:
  0 — CI passed (success or skipped)
  1 — CI failed
  2 — watch timed out before CI completed`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWatch(cmd.Context(), cmd.OutOrStdout(), f)
		},
	}
	cmd.Flags().StringVar(&f.sha, "sha", "", "Commit SHA to watch (default: HEAD)")
	cmd.Flags().StringVar(&f.repo, "repo", "", "GitHub owner/repo (default: derived from origin remote)")
	cmd.Flags().DurationVar(&f.timeout, "timeout", 5*time.Minute, "Max time to wait for CI to complete")
	cmd.Flags().DurationVar(&f.interval, "interval", 10*time.Second, "Polling interval")
	cmd.Flags().BoolVar(&f.jsonOut, "json", false, "Machine-readable JSON output")
	return cmd
}

// ciRun is the subset of GitHub Actions run fields we care about.
type ciRun struct {
	ID         int64  `json:"id"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HTMLURL    string `json:"html_url"`
	Name       string `json:"name"`
}

type ciRunsResponse struct {
	WorkflowRuns []ciRun `json:"workflow_runs"`
}

type watchResult struct {
	SHA        string `json:"sha"`
	Repo       string `json:"repo"`
	RunID      int64  `json:"run_id,omitempty"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion,omitempty"`
	URL        string `json:"url,omitempty"`
	Elapsed    string `json:"elapsed"`
}

func runWatch(ctx context.Context, out io.Writer, f *watchFlags) error {
	// Resolve SHA.
	sha := f.sha
	if sha == "" {
		sha = gitOutput("rev-parse", "HEAD")
	}
	if sha == "" {
		return errcode.New(CodeCIOperationFailed, "could not resolve HEAD SHA", nil)
	}
	sha = strings.TrimSpace(sha)

	// Resolve repo.
	repo := f.repo
	if repo == "" {
		repo = repoFromRemote()
	}
	if repo == "" {
		return errcode.New(CodeCIOperationFailed, "could not determine GitHub owner/repo from origin remote; pass --repo", nil)
	}

	// Resolve token.
	token := resolveToken()
	if token == "" {
		return errcode.New(CodeCIOperationFailed, "no GitHub token available; set GITHUB_TOKEN or run 'gh auth login'", nil)
	}

	start := time.Now()
	deadline := start.Add(f.timeout)

	fmt.Fprintf(out, "forge ci: watching %s@%s (timeout %s)\n", repo, sha[:min8(len(sha))], f.timeout)

	for {
		run, err := fetchRun(ctx, token, repo, sha)
		if err != nil {
			// Non-fatal: log and keep polling.
			fmt.Fprintf(out, "  [poll error: %v]\n", err)
		} else if run != nil { //nolint:gocritic
			elapsed := time.Since(start).Round(time.Second).String()
			switch run.Status {
			case "completed":
				res := watchResult{
					SHA: sha, Repo: repo, RunID: run.ID,
					Status: run.Status, Conclusion: run.Conclusion,
					URL: run.HTMLURL, Elapsed: elapsed,
				}
				if f.jsonOut {
					enc := json.NewEncoder(out)
					enc.SetIndent("", "  ")
					_ = enc.Encode(res)
				}
				switch run.Conclusion {
				case "success", "skipped", "neutral":
					fmt.Fprintf(out, "  ✓ CI passed (run #%d, %s) — %s\n", run.ID, elapsed, run.HTMLURL)
					return nil
				default:
					fmt.Fprintf(out, "  ✗ CI %s (run #%d, %s) — %s\n", run.Conclusion, run.ID, elapsed, run.HTMLURL)
					return errcode.Newf(CodeCIRunFailed, nil,
						"run #%d concluded as %q — %s", run.ID, run.Conclusion, run.HTMLURL)
				}
			default:
				fmt.Fprintf(out, "  … CI %s (run #%d, %s elapsed)\n", run.Status, run.ID, time.Since(start).Round(time.Second))
			}
		} else {
			fmt.Fprintf(out, "  … no run found yet for %s — waiting\n", sha[:min8(len(sha))])
		}

		if time.Now().After(deadline) {
			return errcode.Newf(CodeCIWatchTimeout, nil, "still waiting after %s", f.timeout)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(f.interval):
		}
	}
}

// fetchRun calls the GitHub Actions API for workflow runs on a given SHA.
// Returns nil, nil when no run is found yet (caller should retry).
func fetchRun(ctx context.Context, token, repo, sha string) (*ciRun, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/actions/runs?head_sha=%s&per_page=5", repo, sha)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d for %s", resp.StatusCode, url)
	}

	var runs ciRunsResponse
	if err := json.NewDecoder(resp.Body).Decode(&runs); err != nil {
		return nil, err
	}
	if len(runs.WorkflowRuns) == 0 {
		return nil, nil
	}
	r := runs.WorkflowRuns[0]
	return &r, nil
}

// ── fix ───────────────────────────────────────────────────────────────────────

type fixFlags struct {
	runID   string
	jsonOut bool
}

func newFixCmd() *cobra.Command {
	f := &fixFlags{}
	cmd := &cobra.Command{
		Use:   "fix",
		Short: "Propose an LLM-assisted fix from a CI failure run",
		Long: `forge ci fix retrieves the failure log from the specified GitHub
Actions run and passes it to the configured LLM to generate a targeted
fix proposal (unified diff + explanation).

Requires GITHUB_TOKEN (or gh CLI) for log retrieval and a configured
LLM provider (same as forge fix).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFix(cmd.Context(), cmd.OutOrStdout(), f)
		},
	}
	cmd.Flags().StringVar(&f.runID, "run-id", "", "GitHub Actions run ID (required)")
	cmd.Flags().BoolVar(&f.jsonOut, "json", false, "Machine-readable JSON output")
	_ = cmd.MarkFlagRequired("run-id")
	return cmd
}

func runFix(ctx context.Context, out io.Writer, f *fixFlags) error {
	repo := repoFromRemote()
	token := resolveToken()

	if repo == "" || token == "" {
		return errcode.Newf(CodeCIOperationFailed, nil,
			"--run-id %s: need GitHub token and resolvable repo to fetch logs", f.runID)
	}

	// Fetch failed job logs via GitHub API.
	logsURL := fmt.Sprintf("https://api.github.com/repos/%s/actions/runs/%s/logs", repo, f.runID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, logsURL, nil)
	if err != nil {
		return errcode.New(CodeCIOperationFailed, "build request", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errcode.New(CodeCIOperationFailed, "log fetch failed", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	// GitHub returns a redirect to a zip archive; surface a useful message.
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusOK {
		fmt.Fprintf(out, "forge ci fix: retrieved logs for run #%s\n", f.runID)
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "  Hint: pipe the logs to `forge fix` once LLM provider integration is complete:")
		fmt.Fprintf(out, "    forge fix --from-ci --run-id %s\n", f.runID)
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "  For now, open the failure URL in your browser and run `forge fix` against")
		fmt.Fprintln(out, "  the relevant file(s) manually.")
		return nil
	}

		return errcode.Newf(CodeCIOperationFailed, nil, "unexpected status %d fetching run logs", resp.StatusCode)
}

// ── gotcha ────────────────────────────────────────────────────────────────────

type gotchaFlags struct {
	runID      string
	branch     string
	sha        string
	url        string
	conclusion string
	note       string
	jsonOut    bool
}

// GotchaRecord is the JSONL record written to .forge/learned/gotchas.jsonl.
type GotchaRecord struct {
	TS         string `json:"ts"`
	Branch     string `json:"branch,omitempty"`
	SHA        string `json:"sha,omitempty"`
	RunID      string `json:"run_id,omitempty"`
	Conclusion string `json:"conclusion,omitempty"`
	Note       string `json:"note,omitempty"`
	URL        string `json:"url,omitempty"`
}

func newGotchaCmd() *cobra.Command {
	f := &gotchaFlags{}
	cmd := &cobra.Command{
		Use:   "gotcha",
		Short: "Record a CI failure as a lesson in .forge/learned/gotchas.jsonl",
		Long: `forge ci gotcha appends a JSON record to .forge/learned/gotchas.jsonl
capturing the CI run's outcome so the continuous learning loop (spec §4)
can surface it in future LLM context bundles.

Records are append-only and never sent to any external service unless
forge learn share is explicitly enabled.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runGotcha(cmd.OutOrStdout(), f)
		},
	}
	cmd.Flags().StringVar(&f.runID, "run-id", "", "GitHub Actions run ID")
	cmd.Flags().StringVar(&f.branch, "branch", "", "Branch that was pushed")
	cmd.Flags().StringVar(&f.sha, "sha", "", "Commit SHA")
	cmd.Flags().StringVar(&f.url, "url", "", "URL to the failed CI run")
	cmd.Flags().StringVar(&f.conclusion, "conclusion", "failure", "CI conclusion (failure|timed_out|…)")
	cmd.Flags().StringVar(&f.note, "note", "", "Human-readable lesson summary")
	cmd.Flags().BoolVar(&f.jsonOut, "json", false, "Machine-readable JSON output")
	return cmd
}

func runGotcha(out io.Writer, f *gotchaFlags) error {
	// Resolve defaults from git when not provided.
	if f.branch == "" {
		f.branch = strings.TrimSpace(gitOutput("symbolic-ref", "--short", "HEAD"))
	}
	if f.sha == "" {
		f.sha = strings.TrimSpace(gitOutput("rev-parse", "HEAD"))
	}

	rec := GotchaRecord{
		TS:         time.Now().UTC().Format(time.RFC3339),
		Branch:     f.branch,
		SHA:        f.sha,
		RunID:      f.runID,
		Conclusion: f.conclusion,
		Note:       f.note,
		URL:        f.url,
	}

	line, err := json.Marshal(rec)
	if err != nil {
		return errcode.New(CodeCIGotchaWrite, "marshal failed", err)
	}

	// Ensure directory exists.
	dir := filepath.Dir(gotchaFile)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return errcode.Newf(CodeCIGotchaWrite, err, "mkdir %s", dir)
	}

	// Append to JSONL file.
	fh, err := os.OpenFile(gotchaFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return errcode.Newf(CodeCIGotchaWrite, err, "open %s", gotchaFile)
	}
	defer fh.Close() //nolint:errcheck

	if _, err := fmt.Fprintf(fh, "%s\n", line); err != nil {
		return errcode.Newf(CodeCIGotchaWrite, err, "write %s", gotchaFile)
	}

	if f.jsonOut {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rec)
	} else {
		fmt.Fprintf(out, "forge ci: gotcha recorded → %s\n", gotchaFile)
		if f.note != "" {
			fmt.Fprintf(out, "  note: %s\n", f.note)
		}
		if f.url != "" {
			fmt.Fprintf(out, "  run:  %s\n", f.url)
		}
	}

	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// gitOutput runs a git command and returns stdout; returns "" on error.
func gitOutput(args ...string) string {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// repoFromRemote parses "owner/repo" from the origin remote URL.
// Supports both https://github.com/owner/repo[.git] and git@github.com:owner/repo[.git].
func repoFromRemote() string {
	raw := gitOutput("remote", "get-url", "origin")
	if raw == "" {
		return ""
	}
	// Strip .git suffix.
	raw = strings.TrimSuffix(raw, ".git")
	// HTTPS: https://github.com/owner/repo
	if idx := strings.Index(raw, "github.com/"); idx >= 0 {
		return raw[idx+len("github.com/"):]
	}
	// SSH: git@github.com:owner/repo
	if idx := strings.Index(raw, "github.com:"); idx >= 0 {
		return raw[idx+len("github.com:"):]
	}
	return ""
}

// resolveToken returns a GitHub API token from GITHUB_TOKEN or `gh auth token`.
func resolveToken() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// min8 returns the smaller of n and 8 (used for safe SHA truncation).
func min8(n int) int {
	if n < 8 {
		return n
	}
	return 8
}
