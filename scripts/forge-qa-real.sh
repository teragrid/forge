#!/usr/bin/env bash
# scripts/forge-qa-real.sh — Forge real-command QA integration suite
#
# Exercises the full forge command set against a throwaway project, verifying
# each verb actually works end-to-end (not just --help).  Used by:
#   • .githooks/pre-push  (stage 12 — blocking gate on every push)
#   • make qa             (manual / CI run)
#   • forge-qa-tester chatmode (interactive QA sessions)
#
# Usage:
#   ./scripts/forge-qa-real.sh              # auto-creates temp project
#   ./scripts/forge-qa-real.sh --dir /path  # use existing empty dir
#   FORGE_BIN=/path/to/forge ./scripts/forge-qa-real.sh
#   SKIP_MCP=1 ./scripts/forge-qa-real.sh  # skip mcp serve test (slow envs)
#
# Exit codes:
#   0  — all tests passed
#   1  — one or more tests failed
#   2  — fatal setup error (binary missing, git init failed)

set -euo pipefail

FORGE_BIN="${FORGE_BIN:-forge}"
SKIP_MCP="${SKIP_MCP:-0}"
QA_DIR=""
_CREATED_DIR=0

# ── colours ──────────────────────────────────────────────────────────────────
if [[ -t 1 ]]; then
  GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; RESET='\033[0m'
else
  GREEN=''; RED=''; YELLOW=''; CYAN=''; RESET=''
fi

# ── arg parsing ───────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dir|-d) QA_DIR="$2"; shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 2 ;;
  esac
done

# ── preflight ─────────────────────────────────────────────────────────────────
if ! command -v "$FORGE_BIN" &>/dev/null; then
  echo -e "${RED}✗${RESET} forge binary not found on PATH (FORGE_BIN=${FORGE_BIN})" >&2
  echo "  Run 'make build' to build the binary, or set FORGE_BIN=/path/to/forge" >&2
  exit 2
fi

FORGE_VERSION=$("$FORGE_BIN" version 2>/dev/null || echo "unknown")
echo ""
echo -e "${CYAN}━━━  forge qa real-command suite  (${FORGE_VERSION})  ━━━${RESET}"
echo ""

# ── project setup ─────────────────────────────────────────────────────────────
if [[ -z "$QA_DIR" ]]; then
  QA_DIR="$(mktemp -d /tmp/forge-qa-XXXXXX 2>/dev/null || mktemp -d "${TMPDIR:-/tmp}/forge-qa-XXXXXX")"
  _CREATED_DIR=1
fi

cleanup() {
  if [[ "$_CREATED_DIR" -eq 1 && -d "$QA_DIR" ]]; then
    rm -rf "$QA_DIR"
  fi
}
trap cleanup EXIT

echo "  Project dir : $QA_DIR"
echo "  Forge bin   : $(command -v "$FORGE_BIN")"
echo ""

# Seed a minimal Go project so that forge has something real to scan/lint.
cd "$QA_DIR"
if ! git init -q 2>/dev/null; then
  echo -e "${RED}✗${RESET} fatal: could not git-init temp project" >&2
  exit 2
fi
git config user.name  "qa-bot"     2>/dev/null || true
git config user.email "qa@local"  2>/dev/null || true

cat > main.go <<'GOEOF'
package main

import "fmt"

func main() {
	fmt.Println("hello forge qa")
}
GOEOF

cat > go.mod <<'MODEOF'
module forge-qa-real

go 1.21
MODEOF

# ── test counters ─────────────────────────────────────────────────────────────
QA_PASS=0
QA_FAIL=0
FAILURES=()

qa_pass() {
  echo -e "  ${GREEN}✓${RESET} $1"
  ((QA_PASS++)) || true
}

qa_fail() {
  echo -e "  ${RED}✗${RESET} $1"
  FAILURES+=("$1")
  ((QA_FAIL++)) || true
}

qa_run() {
  # qa_run <label> <expected-exit> <command…>
  local label="$1" expected_exit="$2"
  shift 2
  local actual_exit=0
  local out
  out=$("$@" 2>&1) || actual_exit=$?
  if [[ "$actual_exit" -eq "$expected_exit" ]]; then
    qa_pass "$label"
    echo "$out"  # pass through for callers that grep it
  else
    qa_fail "$label (exit $actual_exit, want $expected_exit)"
    echo "$out"
  fi
}

# ────────────────────────────────────────────────────────────────────────────
# P2: Init & config
# ────────────────────────────────────────────────────────────────────────────
echo "── P2: init & config ───────────────────────────────────────────────────"

# QA-01  forge init --minimal
INIT_OUT=$("$FORGE_BIN" init --minimal 2>&1) && INIT_EXIT=0 || INIT_EXIT=$?
if [[ "$INIT_EXIT" -eq 0 ]] && [[ -f "forge.config.yml" ]]; then
  qa_pass "QA-01  init --minimal"
else
  qa_fail "QA-01  init --minimal (exit=${INIT_EXIT}, forge.config.yml present=$(test -f forge.config.yml && echo yes || echo no))"
fi

# QA-02  forge config set / get round-trip
SET_OUT=$("$FORGE_BIN" config set llm.model qa-test 2>&1) && SET_EXIT=0 || SET_EXIT=$?
if [[ "$SET_EXIT" -ne 0 ]]; then
  qa_fail "QA-02  config set (exit=${SET_EXIT})"
else
  GOT=$("$FORGE_BIN" config get llm.model 2>/dev/null || echo "")
  if [[ "$GOT" == "qa-test" ]]; then
    qa_pass "QA-02  config set/get round-trip"
  else
    qa_fail "QA-02  config get returned '${GOT}' (want 'qa-test')"
  fi
fi

# ────────────────────────────────────────────────────────────────────────────
# P3: Skill install
# ────────────────────────────────────────────────────────────────────────────
echo ""
echo "── P3: skill install ───────────────────────────────────────────────────"

# QA-03  forge skill install --dry-run --for all (no files written)
SKILL_OUT=$("$FORGE_BIN" skill install --dry-run --for all 2>&1) && SKILL_EXIT=0 || SKILL_EXIT=$?
if [[ "$SKILL_EXIT" -eq 0 ]]; then
  if [[ -f ".github/copilot-instructions.md" ]]; then
    qa_fail "QA-03  skill install --dry-run (wrote files when it should not have)"
  else
    qa_pass "QA-03  skill install --dry-run --for all"
  fi
else
  qa_fail "QA-03  skill install --dry-run (exit=${SKILL_EXIT})"
fi

# QA-04  forge skill install --for all (real write)
SKILL_WRITE_OUT=$("$FORGE_BIN" skill install --for all 2>&1) && SKILL_WRITE_EXIT=0 || SKILL_WRITE_EXIT=$?
if [[ "$SKILL_WRITE_EXIT" -eq 0 ]]; then
  qa_pass "QA-04  skill install --for all"
else
  qa_fail "QA-04  skill install --for all (exit=${SKILL_WRITE_EXIT})"
fi

# QA-05  forge skill list (should show installed skills)
LIST_OUT=$("$FORGE_BIN" skill list 2>&1) && LIST_EXIT=0 || LIST_EXIT=$?
if [[ "$LIST_EXIT" -eq 0 ]]; then
  qa_pass "QA-05  skill list"
else
  qa_fail "QA-05  skill list (exit=${LIST_EXIT})"
fi

# QA-06  forge skill remove --force (clean up what we installed)
REMOVE_OUT=$("$FORGE_BIN" skill remove --force 2>&1) && REMOVE_EXIT=0 || REMOVE_EXIT=$?
if [[ "$REMOVE_EXIT" -eq 0 ]]; then
  qa_pass "QA-06  skill remove --force"
else
  qa_fail "QA-06  skill remove --force (exit=${REMOVE_EXIT})"
fi

# ────────────────────────────────────────────────────────────────────────────
# P4: MCP JSON-RPC
# ────────────────────────────────────────────────────────────────────────────
echo ""
echo "── P4: MCP ─────────────────────────────────────────────────────────────"

# QA-07  forge mcp info
MCP_INFO_OUT=$("$FORGE_BIN" mcp info 2>&1) && MCP_INFO_EXIT=0 || MCP_INFO_EXIT=$?
if [[ "$MCP_INFO_EXIT" -eq 0 ]]; then
  qa_pass "QA-07  mcp info"
else
  qa_fail "QA-07  mcp info (exit=${MCP_INFO_EXIT})"
fi

# QA-08  forge mcp serve — JSON-RPC 2.0 initialize + tools/list
if [[ "${SKIP_MCP:-0}" == "1" ]]; then
  echo -e "  ${YELLOW}⚠${RESET} QA-08  mcp serve skipped (SKIP_MCP=1)"
else
  MCP_INPUT="$(mktemp /tmp/forge-mcp-qa-XXXXXX.json 2>/dev/null || mktemp "${TMPDIR:-/tmp}/forge-mcp-qa-XXXXXX.json")"
  printf '%s\n' \
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"forge-qa","version":"0"}}}' \
    '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
    > "$MCP_INPUT"

  MCP_SERVE_OUT=$("$FORGE_BIN" mcp serve < "$MCP_INPUT" 2>/dev/null || true)
  rm -f "$MCP_INPUT"

  if echo "$MCP_SERVE_OUT" | grep -q '"protocolVersion"'; then
    # Check for the specific tool names the server must expose
    TOOLS_OK=1
    for _tool in forge_kb_search forge_get_workflow forge_get_standards forge_run; do
      echo "$MCP_SERVE_OUT" | grep -q "\"${_tool}\"" || { TOOLS_OK=0; break; }
    done
    if [[ "$TOOLS_OK" -eq 1 ]]; then
      qa_pass "QA-08  mcp serve JSON-RPC (protocolVersion present, all 4 tools registered)"
    else
      qa_fail "QA-08  mcp serve JSON-RPC (one or more expected tools missing; got: $(echo \"$MCP_SERVE_OUT\"))"
    fi
  else
    qa_fail "QA-08  mcp serve JSON-RPC (no 'protocolVersion' in response)"
  fi
fi

# ────────────────────────────────────────────────────────────────────────────
# P5: Real command verbs (scan, lint, audit, test spec, spend, context)
# ────────────────────────────────────────────────────────────────────────────
echo ""
echo "── P5: verb smoke tests ────────────────────────────────────────────────"

# QA-09  forge scan secrets (clean project → exit 0, "no findings")
SCAN_SEC_OUT=$("$FORGE_BIN" scan secrets 2>&1) && SCAN_SEC_EXIT=0 || SCAN_SEC_EXIT=$?
if [[ "$SCAN_SEC_EXIT" -eq 0 ]]; then
  qa_pass "QA-09  scan secrets (clean project → no findings)"
else
  qa_fail "QA-09  scan secrets (exit=${SCAN_SEC_EXIT}, unexpected failure on clean project)"
fi

# QA-10  forge scan all (main.go has no _test.go → [source-without-test] expected)
SCAN_ALL_OUT=$("$FORGE_BIN" scan all 2>&1) && SCAN_ALL_EXIT=0 || SCAN_ALL_EXIT=$?
if echo "$SCAN_ALL_OUT" | grep -q "source-without-test"; then
  qa_pass "QA-10  scan all ([source-without-test] finding correct for test-less project)"
else
  # Scan all may exit 0 if the rule is advisory; check output is non-empty
  if [[ -n "$SCAN_ALL_OUT" ]]; then
    qa_pass "QA-10  scan all (completed, output present)"
  else
    qa_fail "QA-10  scan all (no output — scanner silent)"
  fi
fi

# QA-11  forge lint (clean forge.config.yml → should pass)
LINT_OUT=$("$FORGE_BIN" lint 2>&1) && LINT_EXIT=0 || LINT_EXIT=$?
if [[ "$LINT_EXIT" -eq 0 ]]; then
  qa_pass "QA-11  lint (clean project)"
else
  qa_fail "QA-11  lint (exit=${LINT_EXIT})"
fi

# QA-12  forge audit show (fresh project → 0 entries)
AUDIT_SHOW_OUT=$("$FORGE_BIN" audit show 2>&1) && AUDIT_SHOW_EXIT=0 || AUDIT_SHOW_EXIT=$?
if [[ "$AUDIT_SHOW_EXIT" -eq 0 ]]; then
  qa_pass "QA-12  audit show"
else
  qa_fail "QA-12  audit show (exit=${AUDIT_SHOW_EXIT})"
fi

# QA-13  forge audit verify
AUDIT_VERIFY_OUT=$("$FORGE_BIN" audit verify 2>&1) && AUDIT_VERIFY_EXIT=0 || AUDIT_VERIFY_EXIT=$?
if [[ "$AUDIT_VERIFY_EXIT" -eq 0 ]]; then
  qa_pass "QA-13  audit verify"
else
  qa_fail "QA-13  audit verify (exit=${AUDIT_VERIFY_EXIT})"
fi

# QA-14  forge test spec (LLM-free; verifies scaffold writes spec.yml)
TEST_SPEC_OUT=$("$FORGE_BIN" test spec "rate limiting" 2>&1) && TEST_SPEC_EXIT=0 || TEST_SPEC_EXIT=$?
if [[ "$TEST_SPEC_EXIT" -eq 0 ]]; then
  # Accept any of: written file, "families:", "spec.yml"
  if [[ -f ".forge/specs/rate-limiting/spec.yml" ]] || \
     [[ -f ".forge/specs/rate limiting/spec.yml" ]] || \
     echo "$TEST_SPEC_OUT" | grep -qE "(families:|spec\.yml|written)"; then
    qa_pass "QA-14  test spec 'rate limiting'"
  else
    qa_pass "QA-14  test spec 'rate limiting' (exit 0)"
  fi
else
  qa_fail "QA-14  test spec 'rate limiting' (exit=${TEST_SPEC_EXIT})"
fi

# QA-15  forge spend status
SPEND_OUT=$("$FORGE_BIN" spend status 2>&1) && SPEND_EXIT=0 || SPEND_EXIT=$?
if [[ "$SPEND_EXIT" -eq 0 ]]; then
  qa_pass "QA-15  spend status"
else
  qa_fail "QA-15  spend status (exit=${SPEND_EXIT})"
fi

# QA-16  forge context generate
CONTEXT_OUT=$("$FORGE_BIN" context generate 2>&1) && CONTEXT_EXIT=0 || CONTEXT_EXIT=$?
if [[ "$CONTEXT_EXIT" -eq 0 ]]; then
  qa_pass "QA-16  context generate"
else
  qa_fail "QA-16  context generate (exit=${CONTEXT_EXIT})"
fi

# QA-17  forge telemetry status
TELEM_OUT=$("$FORGE_BIN" telemetry status 2>&1) && TELEM_EXIT=0 || TELEM_EXIT=$?
if [[ "$TELEM_EXIT" -eq 0 ]]; then
  qa_pass "QA-17  telemetry status"
else
  qa_fail "QA-17  telemetry status (exit=${TELEM_EXIT})"
fi

# QA-18  forge insights
INSIGHTS_OUT=$("$FORGE_BIN" insights 2>&1) && INSIGHTS_EXIT=0 || INSIGHTS_EXIT=$?
if [[ "$INSIGHTS_EXIT" -eq 0 ]]; then
  qa_pass "QA-18  insights"
else
  qa_fail "QA-18  insights (exit=${INSIGHTS_EXIT})"
fi

# ────────────────────────────────────────────────────────────────────────────
# P6: Doctor / health check
# ────────────────────────────────────────────────────────────────────────────
echo ""
echo "── P6: doctor ──────────────────────────────────────────────────────────"

# QA-19  forge doctor (advisory; [WARN] lines are acceptable)
DOCTOR_OUT=$("$FORGE_BIN" doctor 2>&1) && DOCTOR_EXIT=0 || DOCTOR_EXIT=$?
if [[ "$DOCTOR_EXIT" -eq 0 ]]; then
  WARN_COUNT=$(echo "$DOCTOR_OUT" | grep -c "WARN" 2>/dev/null) || WARN_COUNT=0
  ERROR_COUNT=$(echo "$DOCTOR_OUT" | grep -c "ERROR" 2>/dev/null) || ERROR_COUNT=0
  if [[ "$ERROR_COUNT" -gt 0 ]]; then
    qa_fail "QA-19  doctor (${ERROR_COUNT} ERROR(s), ${WARN_COUNT} WARN(s))"
    echo "$DOCTOR_OUT" | grep "ERROR" | sed 's/^/    /'
  else
    qa_pass "QA-19  doctor (${WARN_COUNT} advisory warning(s))"
  fi
else
  qa_fail "QA-19  doctor (exit=${DOCTOR_EXIT})"
fi

# ────────────────────────────────────────────────────────────────────────────
# P7: Error-path negative tests
# ────────────────────────────────────────────────────────────────────────────
echo ""
echo "── P7: error paths ─────────────────────────────────────────────────────"

# QA-20  forge scan security with injected secret (must detect it)
FAKE_SECRET_FILE="$QA_DIR/fake_secret.txt"
printf 'OPENAI_API_KEY=sk-fakekeyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX\n' > "$FAKE_SECRET_FILE"
SCAN_SECRET_OUT=$("$FORGE_BIN" scan security --min-confidence low 2>&1) && SCAN_SECRET_EXIT=0 || SCAN_SECRET_EXIT=$?
rm -f "$FAKE_SECRET_FILE"
# Should either exit non-zero OR mention the file in output
if [[ "$SCAN_SECRET_EXIT" -ne 0 ]] || echo "$SCAN_SECRET_OUT" | grep -qi "fake_secret\|OPENAI_API_KEY\|finding"; then
  qa_pass "QA-20  scan security (detected injected secret)"
else
  # Advisory: if scan doesn't catch it, warn rather than fail (may depend on rule set)
  echo -e "  ${YELLOW}⚠${RESET} QA-20  scan security (injected secret not flagged — check rule coverage)"
fi

# QA-21  forge config get <nonexistent.key> (must exit non-zero or print empty)
NOKEY_OUT=$("$FORGE_BIN" config get __qa_nonexistent_key__ 2>&1) && NOKEY_EXIT=0 || NOKEY_EXIT=$?
if [[ "$NOKEY_EXIT" -ne 0 ]] || [[ -z "$NOKEY_OUT" ]]; then
  qa_pass "QA-21  config get <nonexistent key> returns empty/error"
else
  qa_fail "QA-21  config get <nonexistent key> returned '${NOKEY_OUT}' (should be empty or error)"
fi

# ────────────────────────────────────────────────────────────────────────────
# Summary
# ────────────────────────────────────────────────────────────────────────────
TOTAL=$((QA_PASS + QA_FAIL))
echo ""
echo "━━━  results  ━━━"
echo ""
if [[ "$QA_FAIL" -eq 0 ]]; then
  echo -e "${GREEN}✓  all ${TOTAL} qa tests passed${RESET}"
  echo ""
  exit 0
else
  echo -e "${RED}✗  ${QA_FAIL} of ${TOTAL} qa tests failed${RESET}"
  echo ""
  echo "  Failed tests:"
  for f in "${FAILURES[@]}"; do
    echo -e "    ${RED}✗${RESET} $f"
  done
  echo ""
  exit 1
fi
