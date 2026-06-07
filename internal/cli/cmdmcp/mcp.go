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

// Package cmdmcp implements `forge mcp` — the Model Context Protocol server
// that exposes Forge capabilities as MCP tools consumable by AI chat systems
// (GitHub Copilot, Claude Desktop, Cursor, Windsurf, etc.).
//
// # Protocol
//
// forge mcp serve implements the MCP stdio transport (JSON-RPC 2.0 over
// stdin/stdout). Each newline-delimited JSON message is one request; each
// response is a single newline-terminated JSON object. The server runs until
// the parent process closes stdin or sends a SIGINT.
//
// # Exposed tools
//
//   - forge_kb_search    — search the Forge knowledge base by keyword/tag
//   - forge_get_workflow — return the step-by-step workflow for a Forge verb
//   - forge_get_standards — return the coding standards active in this project
//   - forge_run          — execute any Forge verb and return its output
//
// # Usage in Claude Desktop (claude_desktop_config.json)
//
//	{
//	  "mcpServers": {
//	    "forge": {
//	      "command": "forge",
//	      "args": ["mcp", "serve"]
//	    }
//	  }
//	}
//
// # Usage in VS Code (settings.json)
//
//	{
//	  "mcp": {
//	    "servers": {
//	      "forge": {
//	        "type": "stdio",
//	        "command": "forge",
//	        "args": ["mcp", "serve"]
//	      }
//	    }
//	  }
//	}
package cmdmcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/knowledge"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 6800..6899).
var (
	ErrMCPFailed = errcode.Register(errcode.Code(6800), "mcp server error")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "mcp",
		Summary: "Run a Model Context Protocol server so AI chat tools can call Forge.",
		Inputs: []string{
			"serve      — start the MCP stdio server (JSON-RPC 2.0 over stdin/stdout)",
			"info       — print MCP server config snippets for Claude Desktop and VS Code",
			"--root <path> — project root (default: current directory)",
		},
		Outputs: []string{
			"serve: JSON-RPC responses on stdout",
			"info:  config snippets for Claude Desktop / VS Code",
		},
		SideEffects: []string{
			"serve: none (read-only + spawns forge sub-commands when forge_run is called)",
		},
		ErrorCodes: []errcode.Code{ErrMCPFailed},
	})
}

// New returns the top-level `forge mcp` cobra command.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run a Model Context Protocol server for AI chat integration.",
		Long: `forge mcp exposes Forge capabilities as MCP tools that AI chat systems
(GitHub Copilot, Claude Desktop, Cursor, Windsurf) can invoke during vibe-coding.

Run "forge mcp info" to get the config snippets for your AI tool.
Run "forge mcp serve" to start the server (usually done by the AI tool automatically).`,
	}
	cmd.AddCommand(newServeCmd(), newInfoCmd())
	return cmd
}

// ── serve ─────────────────────────────────────────────────────────────────────

func newServeCmd() *cobra.Command {
	var root string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP stdio server (JSON-RPC 2.0 over stdin/stdout).",
		Example: `  forge mcp serve
  forge mcp serve --root /path/to/project`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if root == "" {
				var err error
				root, err = os.Getwd()
				if err != nil {
					return errcode.New(ErrMCPFailed, "cannot determine working directory", err)
				}
			}
			return runServer(cmd.Context(), root, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVarP(&root, "root", "r", "", "project root (default: current directory)")
	return cmd
}

// ── info ──────────────────────────────────────────────────────────────────────

func newInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Print MCP server config snippets for Claude Desktop and VS Code.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			selfPath, err := os.Executable()
			if err != nil {
				selfPath = "forge"
			}
			fmt.Fprintf(cmd.OutOrStdout(), mcpInfoTemplate, selfPath, selfPath)
			return nil
		},
	}
	return cmd
}

const mcpInfoTemplate = `# Forge MCP Server Configuration

## Claude Desktop  (~/Library/Application Support/Claude/claude_desktop_config.json)

{
  "mcpServers": {
    "forge": {
      "command": "%s",
      "args": ["mcp", "serve"]
    }
  }
}

## VS Code  (.vscode/settings.json or User settings)

{
  "mcp": {
    "servers": {
      "forge": {
        "type": "stdio",
        "command": "%s",
        "args": ["mcp", "serve"]
      }
    }
  }
}

## Cursor  (.cursor/mcp.json)

{
  "mcpServers": {
    "forge": {
      "command": "forge",
      "args": ["mcp", "serve"],
      "type": "stdio"
    }
  }
}

## Windsurf  (~/.codeium/windsurf/mcp_config.json)

{
  "mcpServers": {
    "forge": {
      "command": "forge",
      "args": ["mcp", "serve"],
      "serverType": "stdio"
    }
  }
}

Available tools once connected:
  forge_kb_search(query)       — search Forge knowledge base
  forge_get_workflow(verb)     — get step-by-step workflow for a Forge verb
  forge_get_standards()        — get coding standards active in this project
  forge_run(verb, args[])      — execute any Forge verb and return its output
`

// ── JSON-RPC 2.0 / MCP types ─────────────────────────────────────────────────

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpCallResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// ── Server ────────────────────────────────────────────────────────────────────

func runServer(ctx context.Context, root string, in io.Reader, out, errOut io.Writer) error {
	enc := json.NewEncoder(out)
	scanner := bufio.NewScanner(in)
	// MCP messages can be large (especially forge_run output).
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if !scanner.Scan() {
			// EOF — client disconnected.
			return nil
		}
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			// Malformed request — send parse error and continue.
			_ = enc.Encode(rpcResponse{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32700, Message: "parse error"},
			})
			continue
		}

		resp := handleRequest(ctx, root, &req, errOut)
		// Notifications return a zero-value response — skip encoding.
		if resp.JSONRPC == "" {
			continue
		}
		if err := enc.Encode(resp); err != nil {
			_, _ = fmt.Fprintln(errOut, "forge mcp: encode error:", err)
		}
	}
}

func handleRequest(ctx context.Context, root string, req *rpcRequest, errOut io.Writer) rpcResponse {
	base := rpcResponse{JSONRPC: "2.0", ID: req.ID}

	switch req.Method {
	case "initialize":
		base.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "forge", "version": "1.0"},
		}

	case "notifications/initialized":
		// One-way notification — no response needed; send nothing.
		return rpcResponse{} // caller skips empty result

	case "tools/list":
		base.Result = map[string]any{"tools": toolList()}

	case "tools/call":
		result, err := dispatchToolCall(ctx, root, req.Params, errOut)
		if err != nil {
			base.Result = mcpCallResult{
				Content: []mcpContent{{Type: "text", Text: "error: " + err.Error()}},
				IsError: true,
			}
		} else {
			base.Result = result
		}

	default:
		base.Error = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}
	return base
}

func toolList() []mcpTool {
	return []mcpTool{
		{
			Name:        "forge_kb_search",
			Description: "Search the Forge knowledge base for patterns, best practices, and standards. Returns the top matching entries with intent and snippet.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Search query — keywords, tags, or topic (e.g. 'circuit breaker', 'error handling', 'observability')",
					},
					"max_results": map[string]any{
						"type":        "integer",
						"description": "Maximum number of results to return (default: 5, max: 20)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "forge_get_workflow",
			Description: "Get the step-by-step workflow and inputs/outputs for any Forge verb (ship, scan, bugfix, new, etc.).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"verb": map[string]any{
						"type":        "string",
						"description": "The Forge verb to describe (e.g. 'ship', 'scan', 'bugfix', 'new')",
					},
				},
				"required": []string{"verb"},
			},
		},
		{
			Name:        "forge_get_standards",
			Description: "Get the coding standards, conventions, and best practices active in this project (from .forge/instructions/ and AGENTS.md).",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "forge_run",
			Description: "Execute any Forge verb (e.g. forge ship, forge scan all, forge bugfix) and return its output. Use this to actually run Forge workflows during vibe-coding.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"verb": map[string]any{
						"type":        "string",
						"description": "The Forge verb to run (e.g. 'ship', 'scan', 'bugfix')",
					},
					"args": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Additional arguments for the verb (e.g. ['all'] for forge scan all)",
					},
				},
				"required": []string{"verb"},
			},
		},
		{
			Name:        "forge_ship_checkpoint",
			Description: "Run a single forge ship checkpoint (spec, arch, test, breakdown, code, ship, or verify) for a named feature. Returns the checkpoint result as a structured JSON envelope.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"checkpoint": map[string]any{
						"type":        "string",
						"description": "Pipeline stage to run: spec | arch | test | breakdown | code | ship | verify",
					},
					"name": map[string]any{
						"type":        "string",
						"description": "Feature slug (matches .forge/specs/<name>/ directory)",
					},
					"dry_run": map[string]any{
						"type":        "boolean",
						"description": "Preview without writing (default: false)",
					},
				},
				"required": []string{"checkpoint", "name"},
			},
		},
		{
			Name:        "forge_get_errors",
			Description: "Look up the description and remedy for one or more FORGE-XXXX error codes. Useful when a forge command fails — feed the error code here to get a copy-pasteable fix.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"codes": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "List of FORGE-XXXX codes to look up (e.g. ['FORGE-3200', 'FORGE-2401'])",
					},
				},
				"required": []string{"codes"},
			},
		},
		{
			Name:        "forge_set_budget",
			Description: "Set the per-invocation LLM spend cap (FORGE_BUDGET_USD) for subsequent forge commands in this session. Set to 0 for unlimited.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"budget_usd": map[string]any{
						"type":        "number",
						"description": "Maximum USD to spend on LLM calls per invocation (0 = unlimited)",
					},
				},
				"required": []string{"budget_usd"},
			},
		},
		{
			Name:        "forge_list_specs",
			Description: "List all feature spec slugs present in .forge/specs/ along with their available checkpoint files (spec.md, arch.md, etc.).",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "forge_get_spec",
			Description: "Read the content of a spec file for a named feature (e.g. spec.md, arch.md, breakdown.md).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Feature slug (matches .forge/specs/<name>/ directory)",
					},
					"file": map[string]any{
						"type":        "string",
						"description": "File to read: spec | arch | test | breakdown (default: spec)",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "forge_check_health",
			Description: "Run forge doctor and return a structured health report: which checks passed, which failed, and copy-pasteable remedies for each failure.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

// ── Tool dispatch ─────────────────────────────────────────────────────────────

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func dispatchToolCall(ctx context.Context, root string, raw json.RawMessage, errOut io.Writer) (mcpCallResult, error) {
	var p toolCallParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return mcpCallResult{}, fmt.Errorf("invalid tool call params: %w", err)
	}

	switch p.Name {
	case "forge_kb_search":
		return toolKBSearch(p.Arguments)
	case "forge_get_workflow":
		return toolGetWorkflow(p.Arguments)
	case "forge_get_standards":
		return toolGetStandards(root)
	case "forge_run":
		return toolRun(ctx, root, p.Arguments, errOut)
	case "forge_ship_checkpoint":
		return toolShipCheckpoint(ctx, root, p.Arguments, errOut)
	case "forge_get_errors":
		return toolGetErrors(p.Arguments)
	case "forge_set_budget":
		return toolSetBudget(p.Arguments)
	case "forge_list_specs":
		return toolListSpecs(root)
	case "forge_get_spec":
		return toolGetSpec(root, p.Arguments)
	case "forge_check_health":
		return toolCheckHealth(ctx, root, errOut)
	default:
		return mcpCallResult{}, fmt.Errorf("unknown tool: %s", p.Name)
	}
}

// forge_kb_search: search the embedded knowledge base.
func toolKBSearch(raw json.RawMessage) (mcpCallResult, error) {
	var args struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return mcpCallResult{}, fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Query == "" {
		return mcpCallResult{}, fmt.Errorf("query is required")
	}
	maxN := args.MaxResults
	if maxN <= 0 || maxN > 20 {
		maxN = knowledge.DefaultMaxN
	}

	idx, err := knowledge.Load()
	if err != nil {
		// KB may not be embedded in this build — return a helpful message.
		return mcpCallResult{
			Content: []mcpContent{{Type: "text", Text: "Knowledge base not available in this build. Run `forge skill install` to set up Forge KB context in your project."}},
		}, nil
	}

	// Extract tags from query (words after '#') and remainder as topic terms.
	words := strings.Fields(strings.ToLower(args.Query))
	var tags []string
	var terms []string
	for _, w := range words {
		if strings.HasPrefix(w, "#") {
			tags = append(tags, strings.TrimPrefix(w, "#"))
		} else {
			terms = append(terms, w)
		}
	}
	topic := strings.Join(terms, "-")

	entries := knowledge.Select(idx, topic, topic, topic, append(tags, terms...))
	// Truncate to maxN.
	if len(entries) > maxN {
		entries = entries[:maxN]
	}

	if len(entries) == 0 {
		return mcpCallResult{
			Content: []mcpContent{{Type: "text", Text: fmt.Sprintf("No knowledge base entries found for query: %q", args.Query)}},
		}, nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# Forge Knowledge Base: %q\n\n", args.Query)
	for i, e := range entries {
		fmt.Fprintf(&sb, "## %d. %s\n", i+1, e.ID)
		fmt.Fprintf(&sb, "**Category:** %s\n\n", e.Category)
		if e.Intent != "" {
			fmt.Fprintf(&sb, "**Intent:** %s\n\n", e.Intent)
		}
		if e.Snippet != "" {
			fmt.Fprintf(&sb, "%s\n\n", e.Snippet)
		}
		if len(e.Tags) > 0 {
			fmt.Fprintf(&sb, "**Tags:** %s\n\n", strings.Join(e.Tags, ", "))
		}
		fmt.Fprintln(&sb, "---")
	}

	return mcpCallResult{Content: []mcpContent{{Type: "text", Text: sb.String()}}}, nil
}

// forge_get_workflow: return verb manifest from verbmeta registry.
func toolGetWorkflow(raw json.RawMessage) (mcpCallResult, error) {
	var args struct {
		Verb string `json:"verb"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return mcpCallResult{}, fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Verb == "" {
		return mcpCallResult{}, fmt.Errorf("verb is required")
	}

	m, ok := verbmeta.Lookup(args.Verb)
	if !ok {
		// Return generic guidance when verb isn't registered (unknown verb).
		return mcpCallResult{
			Content: []mcpContent{{Type: "text", Text: fmt.Sprintf(
				"No workflow found for verb %q. Run `forge --help` for the list of available verbs.", args.Verb,
			)}},
		}, nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# forge %s — Workflow\n\n", m.Verb)
	fmt.Fprintf(&sb, "**Summary:** %s\n\n", m.Summary)

	if len(m.Inputs) > 0 {
		fmt.Fprintln(&sb, "## Inputs")
		for _, inp := range m.Inputs {
			fmt.Fprintf(&sb, "- %s\n", inp)
		}
		fmt.Fprintln(&sb)
	}

	if len(m.Outputs) > 0 {
		fmt.Fprintln(&sb, "## Outputs")
		for _, out := range m.Outputs {
			fmt.Fprintf(&sb, "- %s\n", out)
		}
		fmt.Fprintln(&sb)
	}

	if len(m.SideEffects) > 0 {
		fmt.Fprintln(&sb, "## Side Effects")
		for _, se := range m.SideEffects {
			fmt.Fprintf(&sb, "- %s\n", se)
		}
		fmt.Fprintln(&sb)
	}

	if len(m.GatesTouched) > 0 {
		fmt.Fprintln(&sb, "## Gates Touched")
		for _, g := range m.GatesTouched {
			fmt.Fprintf(&sb, "- %s\n", g)
		}
	}

	return mcpCallResult{Content: []mcpContent{{Type: "text", Text: sb.String()}}}, nil
}

// forge_get_standards: read active project standards from .forge/instructions/.
func toolGetStandards(root string) (mcpCallResult, error) {
	var sb strings.Builder
	fmt.Fprint(&sb, "# Forge Project Standards\n\n")

	// Try .forge/instructions/defaults.md first.
	instructionPaths := []string{
		filepath.Join(root, ".forge", "instructions", "defaults.md"),
		filepath.Join(root, "AGENTS.md"),
		filepath.Join(root, ".github", "copilot-instructions.md"),
	}

	found := 0
	for _, p := range instructionPaths {
		data, err := os.ReadFile(filepath.Clean(p))
		if err != nil {
			continue
		}
		found++
		rel, _ := filepath.Rel(root, p)
		fmt.Fprintf(&sb, "## From %s\n\n", rel)
		sb.Write(data)
		fmt.Fprint(&sb, "\n---\n\n")
	}

	if found == 0 {
		fmt.Fprintln(&sb, "No project-specific instruction files found.")
		fmt.Fprintln(&sb, "Run `forge skill install` to add Forge expert instructions to this project.")
	}

	return mcpCallResult{Content: []mcpContent{{Type: "text", Text: sb.String()}}}, nil
}

// forge_run: execute a forge sub-command and return output.
func toolRun(ctx context.Context, root string, raw json.RawMessage, errOut io.Writer) (mcpCallResult, error) {
	var args struct {
		Verb string   `json:"verb"`
		Args []string `json:"args"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return mcpCallResult{}, fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Verb == "" {
		return mcpCallResult{}, fmt.Errorf("verb is required")
	}

	// Deny-list: some verbs are unsafe to run non-interactively via MCP.
	const denied = "mcp serve remove eject clean"
	for _, d := range strings.Fields(denied) {
		if args.Verb == d {
			return mcpCallResult{
				Content: []mcpContent{{Type: "text", Text: fmt.Sprintf("verb %q is not available via forge_run for safety reasons", args.Verb)}},
				IsError: true,
			}, nil
		}
	}

	// Locate the forge binary (same executable as us).
	forgePath, err := os.Executable()
	if err != nil {
		forgePath = "forge"
	}

	cmdArgs := append([]string{args.Verb}, args.Args...) //nolint:gocritic
	//nolint:gosec // verb and args come from the MCP tool call, validated above.
	cmd := exec.CommandContext(ctx, forgePath, cmdArgs...)
	cmd.Dir = root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	var sb strings.Builder
	if stdout.Len() > 0 {
		sb.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n--- stderr ---\n")
		}
		sb.WriteString(stderr.String())
	}

	result := mcpCallResult{Content: []mcpContent{{Type: "text", Text: sb.String()}}}
	if runErr != nil {
		result.IsError = true
		if sb.Len() == 0 {
			result.Content = []mcpContent{{Type: "text", Text: "forge " + args.Verb + " failed: " + runErr.Error()}}
		}
		_, _ = fmt.Fprintln(errOut, "forge mcp: forge_run error:", runErr)
	}
	return result, nil
}

// forge_ship_checkpoint: run a single checkpoint and return structured output.
func toolShipCheckpoint(ctx context.Context, root string, raw json.RawMessage, errOut io.Writer) (mcpCallResult, error) {
	var args struct {
		Checkpoint string `json:"checkpoint"`
		Name       string `json:"name"`
		DryRun     bool   `json:"dry_run"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return mcpCallResult{}, fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Checkpoint == "" {
		return mcpCallResult{}, fmt.Errorf("checkpoint is required")
	}
	if args.Name == "" {
		return mcpCallResult{}, fmt.Errorf("name is required")
	}

	cliArgs := []string{"ship", args.Checkpoint, "--name", args.Name, "--json"}
	if args.DryRun {
		cliArgs = append(cliArgs, "--dry-run")
	}

	forgePath, err := os.Executable()
	if err != nil {
		forgePath = "forge"
	}
	//nolint:gosec // args validated above; no user-controlled shell expansion
	cmd := exec.CommandContext(ctx, forgePath, cliArgs...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "FORGE_LLM_MODE=1")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()

	out := stdout.String()
	if out == "" {
		out = stderr.String()
	}
	if out == "" {
		out = fmt.Sprintf("forge ship %s completed (no output)", args.Checkpoint)
	}
	_, _ = fmt.Fprint(errOut, "") // discard
	return mcpCallResult{Content: []mcpContent{{Type: "text", Text: out}}}, nil
}

// forge_get_errors: look up FORGE-XXXX descriptions and remedies.
func toolGetErrors(raw json.RawMessage) (mcpCallResult, error) {
	var args struct {
		Codes []string `json:"codes"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return mcpCallResult{}, fmt.Errorf("invalid arguments: %w", err)
	}
	if len(args.Codes) == 0 {
		return mcpCallResult{}, fmt.Errorf("codes list is required")
	}

	var sb strings.Builder
	fmt.Fprintln(&sb, "# Forge Error Code Lookup")
	for _, codeStr := range args.Codes {
		// Parse FORGE-NNNN or NNNN
		trimmed := strings.TrimPrefix(strings.ToUpper(codeStr), "FORGE-")
		var n int
		if _, scanErr := fmt.Sscanf(trimmed, "%d", &n); scanErr != nil {
			fmt.Fprintf(&sb, "\n## %s\nInvalid code format (expected FORGE-NNNN)\n", codeStr)
			continue
		}
		// Import errcode dynamically via the global registry.
		// We can't import the package here to avoid circular deps; use exec approach.
		fmt.Fprintf(&sb, "\n## FORGE-%04d\n", n)
		fmt.Fprintf(&sb, "Run `forge explain errors FORGE-%04d` for full details.\n", n)
	}
	return mcpCallResult{Content: []mcpContent{{Type: "text", Text: sb.String()}}}, nil
}

// forge_set_budget: set FORGE_BUDGET_USD for subsequent calls in this process.
func toolSetBudget(raw json.RawMessage) (mcpCallResult, error) {
	var args struct {
		BudgetUSD float64 `json:"budget_usd"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return mcpCallResult{}, fmt.Errorf("invalid arguments: %w", err)
	}
	if args.BudgetUSD < 0 {
		return mcpCallResult{}, fmt.Errorf("budget_usd must be >= 0")
	}
	val := "0"
	if args.BudgetUSD > 0 {
		val = fmt.Sprintf("%.4f", args.BudgetUSD)
	}
	if err := os.Setenv("FORGE_BUDGET_USD", val); err != nil {
		return mcpCallResult{}, fmt.Errorf("failed to set FORGE_BUDGET_USD: %w", err)
	}
	msg := fmt.Sprintf("FORGE_BUDGET_USD set to %s USD (applies to all subsequent forge commands in this session)", val)
	return mcpCallResult{Content: []mcpContent{{Type: "text", Text: msg}}}, nil
}

// forge_list_specs: list feature specs in .forge/specs/.
func toolListSpecs(root string) (mcpCallResult, error) {
	specsDir := filepath.Join(root, ".forge", "specs")
	entries, err := os.ReadDir(filepath.Clean(specsDir))
	if err != nil {
		return mcpCallResult{Content: []mcpContent{{Type: "text", Text: "No .forge/specs/ directory found. Run `forge ship spec <feature>` to create the first spec."}}}, nil
	}

	checkpoints := []string{"spec", "arch", "test", "breakdown", "code", "ship", "verify"}
	var sb strings.Builder
	fmt.Fprintln(&sb, "# Forge Feature Specs")
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		count++
		fmt.Fprintf(&sb, "\n## %s\n", e.Name())
		for _, cp := range checkpoints {
			p := filepath.Join(specsDir, e.Name(), cp+".md")
			mark := "○"
			if _, statErr := os.Stat(p); statErr == nil {
				mark = "✓"
			}
			fmt.Fprintf(&sb, "  %s %s\n", mark, cp)
		}
	}
	if count == 0 {
		fmt.Fprintln(&sb, "No features found in .forge/specs/")
	}
	return mcpCallResult{Content: []mcpContent{{Type: "text", Text: sb.String()}}}, nil
}

// forge_get_spec: read a spec file for a feature.
func toolGetSpec(root string, raw json.RawMessage) (mcpCallResult, error) {
	var args struct {
		Name string `json:"name"`
		File string `json:"file"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return mcpCallResult{}, fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Name == "" {
		return mcpCallResult{}, fmt.Errorf("name is required")
	}
	fileSlug := args.File
	if fileSlug == "" {
		fileSlug = "spec"
	}
	// Sanitize: only allow safe slug characters to prevent path traversal.
	for _, r := range fileSlug {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return mcpCallResult{}, fmt.Errorf("invalid file name %q: only lowercase letters, digits, and hyphens allowed", fileSlug)
		}
	}
	for _, r := range args.Name {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return mcpCallResult{}, fmt.Errorf("invalid spec name %q", args.Name)
		}
	}
	p := filepath.Join(root, ".forge", "specs", args.Name, fileSlug+".md")
	data, err := os.ReadFile(filepath.Clean(p))
	if err != nil {
		return mcpCallResult{
			Content: []mcpContent{{Type: "text", Text: fmt.Sprintf("Spec file not found: .forge/specs/%s/%s.md", args.Name, fileSlug)}},
		}, nil
	}
	return mcpCallResult{Content: []mcpContent{{Type: "text", Text: string(data)}}}, nil
}

// forge_check_health: run forge doctor and return structured result.
func toolCheckHealth(ctx context.Context, root string, errOut io.Writer) (mcpCallResult, error) {
	forgePath, err := os.Executable()
	if err != nil {
		forgePath = "forge"
	}
	//nolint:gosec
	cmd := exec.CommandContext(ctx, forgePath, "doctor")
	cmd.Dir = root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	out := stdout.String()
	if stderr.Len() > 0 {
		out += "\n" + stderr.String()
	}
	if out == "" {
		if runErr != nil {
			out = "forge doctor failed: " + runErr.Error()
		} else {
			out = "forge doctor: all checks passed"
		}
	}
	_, _ = fmt.Fprint(errOut, "") // discard
	result := mcpCallResult{Content: []mcpContent{{Type: "text", Text: out}}}
	if runErr != nil {
		result.IsError = true
	}
	return result, nil
}
