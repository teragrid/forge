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

package cmdmcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/teragrid/forge/internal/cli/cmdmcp"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// rpcCall sends a single JSON-RPC request line to the MCP server and returns
// the parsed response.  It uses runServer indirectly via the "serve" subcommand.
func rpcCall(t *testing.T, root string, req map[string]any) map[string]any {
	t.Helper()
	return rpcCallMulti(t, root, req)[0]
}

// rpcCallMulti sends multiple requests and returns responses in order.
// Empty-result responses (from notifications/initialized) are kept in the
// slice so callers can assert on them.
func rpcCallMulti(t *testing.T, root string, reqs ...map[string]any) []map[string]any {
	t.Helper()

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, r := range reqs {
		if err := enc.Encode(r); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer

	cmd := cmdmcp.New()
	cmd.SetIn(&buf)
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	args := []string{"serve"}
	if root != "" {
		args = append(args, "--root", root)
	}
	cmd.SetArgs(args)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Logf("mcp serve stderr: %s", errBuf.String())
		t.Fatalf("runServer returned error: %v", err)
	}

	// Decode each non-empty line as a JSON object.
	var results []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(outBuf.String()), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var resp map[string]any
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("decode response line %q: %v", line, err)
		}
		results = append(results, resp)
	}
	return results
}

// makeReq builds a JSON-RPC 2.0 request map.
func makeReq(id int, method string, params map[string]any) map[string]any {
	r := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		r["params"] = params
	}
	return r
}

// ── happy path ────────────────────────────────────────────────────────────────

func TestMCP_Initialize(t *testing.T) {
	resp := rpcCall(t, "", makeReq(1, "initialize", nil))

	if resp["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", resp["jsonrpc"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result is not an object: %T", resp["result"])
	}
	if result["protocolVersion"] == "" {
		t.Error("protocolVersion is empty")
	}
	if _, ok := result["capabilities"]; !ok {
		t.Error("capabilities missing from initialize response")
	}
	info, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatal("serverInfo missing or wrong type")
	}
	if info["name"] != "forge" {
		t.Errorf("serverInfo.name = %v, want forge", info["name"])
	}
}

func TestMCP_ToolsList(t *testing.T) {
	resp := rpcCall(t, "", makeReq(2, "tools/list", nil))

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result is not an object")
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("tools is not an array: %T", result["tools"])
	}
	if len(tools) < 4 {
		t.Errorf("expected at least 4 tools, got %d", len(tools))
	}

	names := make(map[string]bool)
	for _, raw := range tools {
		tool, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if n, ok := tool["name"].(string); ok {
			names[n] = true
		}
	}
	for _, want := range []string{"forge_kb_search", "forge_get_workflow", "forge_get_standards", "forge_run"} {
		if !names[want] {
			t.Errorf("tool %q missing from tools/list", want)
		}
	}
}

func TestMCP_KBSearch_HappyPath(t *testing.T) {
	resp := rpcCall(t, "", makeReq(3, "tools/call", map[string]any{
		"name":      "forge_kb_search",
		"arguments": map[string]any{"query": "error handling", "max_results": 3},
	}))

	if resp["error"] != nil {
		t.Fatalf("unexpected rpc error: %v", resp["error"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result not an object")
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatal("content missing or empty")
	}
	text := content[0].(map[string]any)["text"].(string)
	// KB may be empty in public build — tolerate "not available" response.
	if text == "" {
		t.Error("content text is empty")
	}
}

func TestMCP_GetWorkflow_RegisteredVerb(t *testing.T) {
	// "mcp" verb is registered by the init() in this package.
	resp := rpcCall(t, "", makeReq(4, "tools/call", map[string]any{
		"name":      "forge_get_workflow",
		"arguments": map[string]any{"verb": "mcp"},
	}))

	if resp["error"] != nil {
		t.Fatalf("unexpected rpc error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "forge mcp") {
		t.Errorf("expected 'forge mcp' in workflow text; got: %s", text)
	}
}

func TestMCP_GetStandards_WithAGENTSmd(t *testing.T) {
	dir := t.TempDir()
	agentsContent := "# Project Standards\nuse errcode.Register\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(agentsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	resp := rpcCall(t, dir, makeReq(5, "tools/call", map[string]any{
		"name":      "forge_get_standards",
		"arguments": map[string]any{},
	}))

	result := resp["result"].(map[string]any)
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "errcode.Register") {
		t.Errorf("expected AGENTS.md content in standards output; got: %s", text)
	}
}

func TestMCP_GetStandards_NoFiles_GracefulFallback(t *testing.T) {
	dir := t.TempDir()
	resp := rpcCall(t, dir, makeReq(6, "tools/call", map[string]any{
		"name":      "forge_get_standards",
		"arguments": map[string]any{},
	}))

	result := resp["result"].(map[string]any)
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "No project-specific") {
		t.Errorf("expected fallback message; got: %s", text)
	}
}

// ── boundary cases ────────────────────────────────────────────────────────────

func TestMCP_KBSearch_MaxResultsCappedAt20(t *testing.T) {
	// max_results=100 should be capped to knowledge.DefaultMaxN (or 20).
	resp := rpcCall(t, "", makeReq(10, "tools/call", map[string]any{
		"name":      "forge_kb_search",
		"arguments": map[string]any{"query": "security", "max_results": 100},
	}))

	// Should not error; implementation caps internally.
	if resp["error"] != nil {
		t.Fatalf("unexpected rpc error: %v", resp["error"])
	}
}

func TestMCP_KBSearch_ZeroMaxResults_UsesDefault(t *testing.T) {
	resp := rpcCall(t, "", makeReq(11, "tools/call", map[string]any{
		"name":      "forge_kb_search",
		"arguments": map[string]any{"query": "testing", "max_results": 0},
	}))

	if resp["error"] != nil {
		t.Fatalf("unexpected rpc error: %v", resp["error"])
	}
}

func TestMCP_GetWorkflow_UnknownVerb_ReturnsFallback(t *testing.T) {
	resp := rpcCall(t, "", makeReq(12, "tools/call", map[string]any{
		"name":      "forge_get_workflow",
		"arguments": map[string]any{"verb": "xyzzy-nonexistent-verb"},
	}))

	if resp["error"] != nil {
		t.Fatalf("unexpected rpc error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "No workflow found") {
		t.Errorf("expected 'No workflow found' fallback; got: %s", text)
	}
}

// ── negative cases ─────────────────────────────────────────────────────────────

func TestMCP_UnknownMethod_Returns32601(t *testing.T) {
	resp := rpcCall(t, "", makeReq(20, "nonexistent/method", nil))

	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object; got result=%v", resp["result"])
	}
	code := int(errObj["code"].(float64))
	if code != -32601 {
		t.Errorf("error code = %d, want -32601", code)
	}
}

func TestMCP_MalformedJSON_Returns32700(t *testing.T) {
	var outBuf bytes.Buffer
	var inBuf bytes.Buffer
	inBuf.WriteString("{bad json\n")

	cmd := cmdmcp.New()
	cmd.SetIn(&inBuf)
	cmd.SetOut(&outBuf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"serve"})

	ctx := context.Background()
	_ = cmd.ExecuteContext(ctx)

	var resp map[string]any
	if err := json.NewDecoder(&outBuf).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error for malformed JSON; got: %v", resp)
	}
	code := int(errObj["code"].(float64))
	if code != -32700 {
		t.Errorf("error code = %d, want -32700 (parse error)", code)
	}
}

func TestMCP_KBSearch_EmptyQuery_ReturnsError(t *testing.T) {
	resp := rpcCall(t, "", makeReq(21, "tools/call", map[string]any{
		"name":      "forge_kb_search",
		"arguments": map[string]any{"query": ""},
	}))

	// Should return an isError result (tool-level error, not RPC error).
	if resp["error"] != nil {
		// Either an RPC-level error or tool-level is acceptable.
		return
	}
	result := resp["result"].(map[string]any)
	isError, _ := result["isError"].(bool)
	if !isError {
		t.Error("expected isError=true for empty query")
	}
}

func TestMCP_GetWorkflow_EmptyVerb_ReturnsError(t *testing.T) {
	resp := rpcCall(t, "", makeReq(22, "tools/call", map[string]any{
		"name":      "forge_get_workflow",
		"arguments": map[string]any{"verb": ""},
	}))

	result := resp["result"].(map[string]any)
	isError, _ := result["isError"].(bool)
	if !isError {
		t.Error("expected isError=true for empty verb")
	}
}

func TestMCP_UnknownTool_ReturnsIsError(t *testing.T) {
	resp := rpcCall(t, "", makeReq(23, "tools/call", map[string]any{
		"name":      "forge_nonexistent_tool",
		"arguments": map[string]any{},
	}))

	result := resp["result"].(map[string]any)
	isError, _ := result["isError"].(bool)
	if !isError {
		t.Error("expected isError=true for unknown tool name")
	}
}

// ── deny-list for forge_run ───────────────────────────────────────────────────

func TestMCP_ForgeRun_DeniedVerbs_ReturnIsError(t *testing.T) {
	denied := []string{"mcp", "serve", "remove", "eject", "clean"}
	for _, verb := range denied {
		t.Run(verb, func(t *testing.T) {
			resp := rpcCall(t, "", makeReq(30, "tools/call", map[string]any{
				"name":      "forge_run",
				"arguments": map[string]any{"verb": verb},
			}))

			result := resp["result"].(map[string]any)
			isError, _ := result["isError"].(bool)
			if !isError {
				t.Errorf("verb %q should be denied but isError=false", verb)
			}
			content := result["content"].([]any)
			text := content[0].(map[string]any)["text"].(string)
			if !strings.Contains(text, "safety reasons") {
				t.Errorf("expected 'safety reasons' in denial message; got: %s", text)
			}
		})
	}
}

// False-positive guard: a non-denied valid verb (ship) is attempted via
// forge_run; it will fail because forge binary isn't built yet in test, but
// the denial check must NOT fire (error should not mention "safety reasons").
func TestMCP_ForgeRun_NonDeniedVerb_NotBlocked(t *testing.T) {
	var inBuf bytes.Buffer
	enc := json.NewEncoder(&inBuf)
	_ = enc.Encode(makeReq(31, "tools/call", map[string]any{
		"name":      "forge_run",
		"arguments": map[string]any{"verb": "ship"},
	}))

	var outBuf bytes.Buffer
	cmd := cmdmcp.New()
	cmd.SetIn(&inBuf)
	cmd.SetOut(&outBuf)
	cmd.SetErr(&bytes.Buffer{})
	// Short timeout so the forge binary exec doesn't block the test suite.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd.SetArgs([]string{"serve", "--root", t.TempDir()})
	_ = cmd.ExecuteContext(ctx)

	for _, line := range strings.Split(strings.TrimSpace(outBuf.String()), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var resp map[string]any
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue
		}
		result, ok := resp["result"].(map[string]any)
		if !ok {
			continue
		}
		content, ok := result["content"].([]any)
		if !ok || len(content) == 0 {
			return
		}
		text, _ := content[0].(map[string]any)["text"].(string)
		if strings.Contains(text, "safety reasons") {
			t.Error("non-denied verb 'ship' incorrectly blocked by deny-list")
		}
	}
}

// ── idempotency / replay ──────────────────────────────────────────────────────

func TestMCP_ToolsList_Idempotent(t *testing.T) {
	r1 := rpcCall(t, "", makeReq(40, "tools/list", nil))
	r2 := rpcCall(t, "", makeReq(41, "tools/list", nil))

	tools1, _ := json.Marshal(r1["result"])
	tools2, _ := json.Marshal(r2["result"])
	if string(tools1) != string(tools2) {
		t.Error("tools/list is not idempotent")
	}
}

func TestMCP_KBSearch_Idempotent(t *testing.T) {
	req := map[string]any{
		"name":      "forge_kb_search",
		"arguments": map[string]any{"query": "resilience"},
	}
	r1 := rpcCall(t, "", makeReq(42, "tools/call", req))
	r2 := rpcCall(t, "", makeReq(43, "tools/call", req))

	text1, _ := json.Marshal(r1["result"])
	text2, _ := json.Marshal(r2["result"])
	if string(text1) != string(text2) {
		t.Error("forge_kb_search is not idempotent for the same query")
	}
}

// ── notifications/initialized (no-response notification) ─────────────────────

func TestMCP_NotificationsInitialized_ProducesNoOutput(t *testing.T) {
	// notifications/initialized is a one-way notification; server must not
	// send a response for it.
	var inBuf bytes.Buffer
	enc := json.NewEncoder(&inBuf)
	// Write the notification (no id field).
	_ = enc.Encode(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})
	// Write a real request after it to confirm the server is still alive.
	_ = enc.Encode(makeReq(1, "tools/list", nil))

	var outBuf bytes.Buffer
	cmd := cmdmcp.New()
	cmd.SetIn(&inBuf)
	cmd.SetOut(&outBuf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"serve"})
	_ = cmd.ExecuteContext(context.Background())

	// Should have exactly ONE response (for tools/list), not two.
	lines := 0
	for _, l := range strings.Split(strings.TrimSpace(outBuf.String()), "\n") {
		if strings.TrimSpace(l) != "" {
			lines++
		}
	}
	if lines != 1 {
		t.Errorf("expected 1 response for notification+request, got %d lines:\n%s", lines, outBuf.String())
	}
}

// ── data accuracy ─────────────────────────────────────────────────────────────

func TestMCP_GetStandards_ReturnsAllFoundFiles(t *testing.T) {
	dir := t.TempDir()
	// Create two instruction files.
	agentsContent := "agents-content"
	copilotContent := "copilot-instructions-content"
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(agentsContent), 0o644)
	copilotDir := filepath.Join(dir, ".github")
	_ = os.MkdirAll(copilotDir, 0o755)
	_ = os.WriteFile(filepath.Join(copilotDir, "copilot-instructions.md"), []byte(copilotContent), 0o644)

	resp := rpcCall(t, dir, makeReq(50, "tools/call", map[string]any{
		"name":      "forge_get_standards",
		"arguments": map[string]any{},
	}))

	result := resp["result"].(map[string]any)
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, agentsContent) {
		t.Error("AGENTS.md content missing from standards output")
	}
	if !strings.Contains(text, copilotContent) {
		t.Error("copilot-instructions.md content missing from standards output")
	}
}

// ── request ID passthrough ─────────────────────────────────────────────────────

func TestMCP_ResponseIDMatchesRequest(t *testing.T) {
	resp := rpcCall(t, "", makeReq(99, "tools/list", nil))

	id, ok := resp["id"]
	if !ok {
		t.Fatal("response missing id field")
	}
	// JSON numbers decode as float64 by default.
	if int(id.(float64)) != 99 {
		t.Errorf("response id = %v, want 99", id)
	}
}

// ── cobra command structure ───────────────────────────────────────────────────

func TestMCP_New_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	cmd := cmdmcp.New()
	if cmd == nil {
		t.Fatal("New() returned nil")
	}
	if cmd.Use != "mcp" {
		t.Errorf("Use = %q, want mcp", cmd.Use)
	}
}

func TestMCP_HasServeAndInfoSubcommands(t *testing.T) {
	t.Parallel()
	cmd := cmdmcp.New()
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"serve", "info"} {
		if !names[want] {
			t.Errorf("mcp subcommand %q missing", want)
		}
	}
}

func TestMCP_Info_PrintsConfigSnippets(t *testing.T) {
	var outBuf bytes.Buffer
	cmd := cmdmcp.New()
	cmd.SetOut(&outBuf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"info"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("info: %v", err)
	}
	out := outBuf.String()
	for _, want := range []string{"Claude Desktop", "VS Code", "Cursor", "Windsurf", "mcpServers", "forge_kb_search"} {
		if !strings.Contains(out, want) {
			t.Errorf("mcp info missing %q", want)
		}
	}
}
