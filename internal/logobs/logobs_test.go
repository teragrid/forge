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
package logobs

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// Test design (per always-write-tests.md):
//   Happy: New() returns a working logger; emits one event per call.
//   Boundary: Options{} zero-value works (defaults).
//   Negative: secret_* fields are redacted in default mode.
//   Regression: Explain=true prevents redaction (DEV-M0-04 TC-04-05).
//   False-positive guard: non-secret-keyed values flow unchanged.
//   Data-accuracy: JSON output parses with msg + level + custom field.

func TestNew_DefaultsAndBasicEvent(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l := New(Options{Out: &buf, Format: FormatJSON})
	l.Info("hello", slog.String("k", "v"))
	var ev map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &ev); err != nil {
		t.Fatalf("not JSON: %v: %q", err, buf.String())
	}
	if ev["msg"] != "hello" || ev["level"] != "INFO" || ev["k"] != "v" {
		t.Fatalf("unexpected event: %#v", ev)
	}
}

func TestNew_ZeroValue(t *testing.T) {
	t.Parallel()
	// Should not panic. Output goes to stderr; we just exercise the path.
	l := New(Options{})
	l.Info("zero")
}

func TestRedaction_Default(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l := New(Options{Out: &buf, Format: FormatJSON})
	l.Info("e", slog.String("secret_token", "sk_live_AAAA"), slog.String("safe", "ok"))
	got := buf.String()
	if strings.Contains(got, "sk_live_AAAA") {
		t.Fatalf("secret leaked: %s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("redaction marker missing: %s", got)
	}
	if !strings.Contains(got, `"safe":"ok"`) {
		t.Fatalf("non-secret field altered: %s", got)
	}
}

func TestRedaction_Explain_BypassesRedaction(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l := New(Options{Out: &buf, Format: FormatJSON, Explain: true})
	l.Info("e", slog.String("secret_token", "sk_live_AAAA"))
	if !strings.Contains(buf.String(), "sk_live_AAAA") {
		t.Fatalf("explain mode should not redact: %s", buf.String())
	}
}

func TestCtxRoundTrip(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l := New(Options{Out: &buf, Format: FormatJSON})
	ctx := CtxWithLogger(context.Background(), l)
	if FromCtx(ctx) != l {
		t.Fatal("FromCtx did not return bound logger")
	}
	if FromCtx(context.Background()) == nil {
		t.Fatal("FromCtx default must be non-nil")
	}
}
