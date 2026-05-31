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
// Package telemetry implements file-based observability spans (ADR-006,
// DEV-M3-01). Spans are appended as JSON-Lines to .forge/telemetry.jsonl
// when the user has opted in. No OTLP SDK dependency.
package telemetry

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// DefaultSpanPath is the project-relative path for the span log.
const DefaultSpanPath = ".forge/telemetry.jsonl"

// DefaultConfigPath is the user-scoped telemetry config (opt-in flag + install ID).
const DefaultConfigPath = ".forge/telemetry.json"

// Span holds one verb execution span (no PII).
// Fields marked "OTLP-compatible" align with the OpenTelemetry Trace data
// model so spans can be forwarded to any OTLP-capable backend.
type Span struct {
	TraceID    string `json:"trace_id"`
	SpanID     string `json:"span_id"`
	Verb       string `json:"verb"`
	ExitCode   int    `json:"exit_code"`
	DurationMS int64  `json:"duration_ms"`
	ErrorCode  int    `json:"error_code,omitempty"`
	InstallID  string `json:"install_id"`
	Version    string `json:"version,omitempty"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	Timestamp  string `json:"timestamp"`

	// P2: OTLP-compatible pipeline / checkpoint fields.
	// ParentSpanID links a checkpoint span to its pipeline root span.
	ParentSpanID string `json:"parent_span_id,omitempty"`
	// Checkpoint is the checkpoint name (e.g. "plan", "code", "test").
	Checkpoint string `json:"checkpoint,omitempty"`
	// SpanKind distinguishes pipeline root spans ("pipeline") from
	// per-checkpoint child spans ("checkpoint").
	SpanKind string `json:"span_kind,omitempty"`
	// StatusCode is "OK" or "ERROR" (mirrors OTLP StatusCode enum).
	StatusCode string `json:"status_code,omitempty"`
}

// Config is the persisted telemetry configuration.
type Config struct {
	Enabled   bool   `json:"enabled"`
	InstallID string `json:"install_id"`
}

// defaultVersion is set by ldflags in release builds; otherwise "dev".
var defaultVersion = "dev"

// emitMu guards concurrent writes to the span log.
var emitMu sync.Mutex

// newID generates a 16-byte (32 hex chars) cryptographically random ID.
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b)
}

// LoadConfig reads the telemetry config from path. Missing file → disabled.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{Enabled: false, InstallID: newID()}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("telemetry: read config %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("telemetry: parse config %s: %w", path, err)
	}
	return &cfg, nil
}

// SaveConfig writes cfg to path (parents created, mode 0644).
func SaveConfig(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("telemetry: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

// LoadConfigDefault loads the config from root/.forge/telemetry.json.
func LoadConfigDefault(root string) (*Config, error) {
	return LoadConfig(filepath.Join(root, DefaultConfigPath))
}

// RotateInstallID regenerates the install ID (user-initiated pseudonym rotation).
func RotateInstallID(cfg *Config) {
	cfg.InstallID = newID()
}

// Emit appends a single span to spanPath if telemetry is enabled in cfg.
// It is safe to call from multiple goroutines.
func Emit(spanPath string, cfg *Config, s Span) error {
	if !cfg.Enabled {
		return nil
	}
	s.InstallID = cfg.InstallID
	s.OS = runtime.GOOS
	s.Arch = runtime.GOARCH
	if s.Version == "" {
		s.Version = defaultVersion
	}
	if s.Timestamp == "" {
		s.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if s.TraceID == "" {
		s.TraceID = newID()
	}
	if s.SpanID == "" {
		s.SpanID = newID()
	}

	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("telemetry: marshal span: %w", err)
	}
	data = append(data, '\n')

	emitMu.Lock()
	defer emitMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(spanPath), 0o755); err != nil {
		return fmt.Errorf("telemetry: mkdir: %w", err)
	}
	f, err := os.OpenFile(spanPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("telemetry: open span log: %w", err)
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// ReadSpans reads all spans from spanPath. Missing file returns nil, nil.
func ReadSpans(spanPath string) ([]Span, error) {
	data, err := os.ReadFile(spanPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("telemetry: read spans: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	var spans []Span
	for dec.More() {
		var s Span
		if err := dec.Decode(&s); err != nil {
			return nil, fmt.Errorf("telemetry: decode span: %w", err)
		}
		spans = append(spans, s)
	}
	return spans, nil
}

// StartPipelineSpan emits a root span for a pipeline run and returns the
// traceID and spanID to be forwarded to child checkpoint spans.
// The span is written to root/.forge/telemetry.jsonl if telemetry is enabled.
// Errors are non-fatal: callers always receive a valid traceID/spanID.
func StartPipelineSpan(root, verb string) (traceID, spanID string) {
	traceID = newID()
	spanID = newID()[:16] // 8-byte span ID per OTLP convention
	cfg, err := LoadConfigDefault(root)
	if err != nil || !cfg.Enabled {
		return traceID, spanID
	}
	spanPath := filepath.Join(root, DefaultSpanPath)
	_ = Emit(spanPath, cfg, Span{
		TraceID:    traceID,
		SpanID:     spanID,
		Verb:       verb,
		SpanKind:   "pipeline",
		StatusCode: "OK",
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	})
	return traceID, spanID
}

// EmitCheckpointSpan records a child span for one checkpoint in a pipeline run.
// parentSpanID must be the spanID returned by StartPipelineSpan.
// status should be "OK" or "ERROR".
// dur is the wall-clock duration of the checkpoint.
func EmitCheckpointSpan(root, traceID, parentSpanID, cpName, status string, dur time.Duration) error {
	cfg, err := LoadConfigDefault(root)
	if err != nil || !cfg.Enabled {
		return nil // telemetry off — silently skip
	}
	spanPath := filepath.Join(root, DefaultSpanPath)
	return Emit(spanPath, cfg, Span{
		TraceID:      traceID,
		SpanID:       newID()[:16],
		ParentSpanID: parentSpanID,
		Verb:         "ship",
		Checkpoint:   cpName,
		SpanKind:     "checkpoint",
		StatusCode:   status,
		DurationMS:   dur.Milliseconds(),
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
	})
}
