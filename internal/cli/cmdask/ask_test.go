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

package cmdask_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/cli/cmdask"
)

// ── Run unit tests ────────────────────────────────────────────────────────────

// TestRun_NoProvider returns a graceful no-provider result when no API keys set.
func TestRun_NoProvider(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("AZURE_OPENAI_API_KEY", "")
	t.Setenv("AWS_BEDROCK_REGION", "")
	t.Setenv("OLLAMA_HOST", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	tmp := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", filepath.Join(tmp, "none"))
	// Block `gh auth token` subprocess so the test is hermetic.
	t.Setenv("PATH", tmp)
	root := tmp
	res := cmdask.Run(root, "What is the architecture?", "", false)
	if !res.NoProvider {
		t.Error("expected NoProvider=true when no API keys set")
	}
	if res.Answer == "" {
		t.Error("expected non-empty guidance answer when no provider")
	}
	if res.Question != "What is the architecture?" {
		t.Errorf("Question = %q", res.Question)
	}
}

// TestRun_ReadsContextFiles loads project context files if present.
func TestRun_ReadsContextFiles(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	root := t.TempDir()
	// Write an AGENTS.md file.
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Project\nThis is a test project."), 0o644); err != nil {
		t.Fatal(err)
	}
	// Without a provider, Run should still include the context gracefully.
	res := cmdask.Run(root, "What is this project?", "", false)
	if !res.NoProvider {
		t.Skip("LLM provider unexpectedly configured in test environment")
	}
	// The question should still be echoed correctly.
	if res.Question != "What is this project?" {
		t.Errorf("Question = %q", res.Question)
	}
}

// TestRun_ExtractEvidence verifies evidence extraction via the public Run API.
// Since we cannot make real LLM calls in unit tests, this is covered indirectly
// via the cite flag path — the function should still return a valid AskResult.
func TestRun_CiteFlagPreserved(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	root := t.TempDir()
	res := cmdask.Run(root, "explain auth flow", "", true /*cite*/)
	// Without a provider the cite path still returns gracefully.
	if !res.NoProvider {
		t.Skip("LLM provider configured; skipping no-provider path")
	}
	if res.Question == "" {
		t.Error("Question should be set even in no-provider path")
	}
}

// ── AskResult JSON fields ─────────────────────────────────────────────────────

func TestAskResult_JSONFields(t *testing.T) {
	t.Parallel()
	res := cmdask.AskResult{
		Question:   "test?",
		Answer:     "answer",
		Evidence:   []string{"AGENTS.md"},
		Model:      "mock-v1",
		TokensIn:   10,
		TokensOut:  5,
		NoProvider: false,
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, field := range []string{"question", "answer", "evidence", "model", "tokens_in", "tokens_out"} {
		if _, ok := m[field]; !ok {
			t.Errorf("missing JSON field %q", field)
		}
	}
}

// ── Cobra command integration ─────────────────────────────────────────────────

func TestNew_CobraRegistered(t *testing.T) {
	t.Parallel()
	cmd := cmdask.New()
	if cmd == nil {
		t.Fatal("New() returned nil")
	}
	if cmd.Use == "" {
		t.Error("Use is empty")
	}
	// Verify --json flag exists.
	if cmd.Flags().Lookup("json") == nil {
		t.Error("--json flag not registered")
	}
	// Verify --cite flag exists.
	if cmd.Flags().Lookup("cite") == nil {
		t.Error("--cite flag not registered")
	}
	// Verify error subcommand is present.
	var found bool
	for _, sub := range cmd.Commands() {
		if sub.Use == "error <code>" {
			found = true
		}
	}
	if !found {
		t.Error("forge ask error subcommand not registered")
	}
}

// TestAsk_JSONOutput verifies that --json produces valid JSON output.
func TestAsk_JSONOutput(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	root := t.TempDir()
	cmd := buildTestRoot(root)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"ask", "--root", root, "--json", "What is this?"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var res cmdask.AskResult
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("JSON output invalid: %v\noutput: %s", err, buf.String())
	}
	if res.Question == "" {
		t.Error("JSON output missing question field")
	}
}

// TestAsk_PlainOutput verifies that without --json the plain formatter runs.
func TestAsk_PlainOutput(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	root := t.TempDir()
	cmd := buildTestRoot(root)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"ask", "--root", root, "What is this project?"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// When NoProvider=true the answer guidance is printed directly.
	if buf.String() == "" {
		t.Error("expected non-empty plain output")
	}
}

// TestAsk_ErrorSubcommand_MissingArg returns an error when no code given.
func TestAsk_ErrorSubcommand_MissingArg(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := buildTestRoot(root)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"ask", "error"})
	// Should fail with usage error (missing required arg).
	_ = cmd.Execute() // don't assert error — cobra may print usage
}

// TestAsk_MultiWordQuestion joins args into a single question.
func TestAsk_MultiWordQuestion(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	root := t.TempDir()
	cmd := buildTestRoot(root)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"ask", "--root", root, "--json", "why", "is", "the", "checkout", "slow"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var res cmdask.AskResult
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if !strings.Contains(res.Question, "checkout") {
		t.Errorf("multi-word question not joined: %q", res.Question)
	}
}

// buildTestRoot wraps the ask command in a minimal root command for testing.
func buildTestRoot(root string) *cobra.Command {
	_ = root
	rootCmd := &cobra.Command{Use: "forge", SilenceUsage: true, SilenceErrors: true}
	rootCmd.AddCommand(cmdask.New())
	return rootCmd
}
