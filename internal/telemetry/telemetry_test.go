package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TC-TEL-01: LoadConfig on missing file returns disabled config with install ID.
func TestLoadConfig_MissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(filepath.Join(dir, "telemetry.json"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Enabled {
		t.Error("want disabled for missing file")
	}
	if cfg.InstallID == "" {
		t.Error("want non-empty install ID")
	}
}

// TC-TEL-02: SaveConfig + LoadConfig round-trip.
func TestConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.json")
	cfg := &Config{Enabled: true, InstallID: "test-id-123"}
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	got, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got.Enabled != true {
		t.Error("Enabled not persisted")
	}
	if got.InstallID != "test-id-123" {
		t.Errorf("InstallID not persisted: %q", got.InstallID)
	}
}

// TC-TEL-03: Emit when disabled writes nothing.
func TestEmit_Disabled(t *testing.T) {
	dir := t.TempDir()
	spanPath := filepath.Join(dir, "t.jsonl")
	cfg := &Config{Enabled: false, InstallID: "id1"}
	if err := Emit(spanPath, cfg, Span{Verb: "test"}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if _, err := os.Stat(spanPath); !os.IsNotExist(err) {
		t.Error("span log should not be created when disabled")
	}
}

// TC-TEL-04: Emit when enabled writes a span.
func TestEmit_Enabled(t *testing.T) {
	dir := t.TempDir()
	spanPath := filepath.Join(dir, "t.jsonl")
	cfg := &Config{Enabled: true, InstallID: "id2"}
	if err := Emit(spanPath, cfg, Span{Verb: "scan", ExitCode: 0}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	spans, err := ReadSpans(spanPath)
	if err != nil {
		t.Fatalf("ReadSpans: %v", err)
	}
	if len(spans) != 1 {
		t.Fatalf("want 1 span, got %d", len(spans))
	}
	if spans[0].Verb != "scan" {
		t.Errorf("wrong verb: %q", spans[0].Verb)
	}
}

// TC-TEL-05: Multiple Emit calls append (not overwrite).
func TestEmit_Append(t *testing.T) {
	dir := t.TempDir()
	spanPath := filepath.Join(dir, "t.jsonl")
	cfg := &Config{Enabled: true, InstallID: "id3"}
	for i := 0; i < 3; i++ {
		Emit(spanPath, cfg, Span{Verb: "v"}) //nolint:errcheck
	}
	spans, err := ReadSpans(spanPath)
	if err != nil {
		t.Fatalf("ReadSpans: %v", err)
	}
	if len(spans) != 3 {
		t.Fatalf("want 3 spans, got %d", len(spans))
	}
}

// TC-TEL-06: ReadSpans on missing file returns nil, nil.
func TestReadSpans_MissingFile(t *testing.T) {
	spans, err := ReadSpans(filepath.Join(t.TempDir(), "nofile.jsonl"))
	if err != nil {
		t.Fatalf("ReadSpans: %v", err)
	}
	if spans != nil {
		t.Error("want nil for missing file")
	}
}

// TC-TEL-07: Span has InstallID, OS, Arch populated by Emit.
func TestEmit_PopulatesFields(t *testing.T) {
	dir := t.TempDir()
	spanPath := filepath.Join(dir, "t.jsonl")
	cfg := &Config{Enabled: true, InstallID: "myid"}
	Emit(spanPath, cfg, Span{Verb: "check", DurationMS: 42}) //nolint:errcheck
	spans, _ := ReadSpans(spanPath)
	s := spans[0]
	if s.InstallID != "myid" {
		t.Errorf("InstallID: got %q", s.InstallID)
	}
	if s.OS == "" {
		t.Error("OS should be populated")
	}
	if s.Arch == "" {
		t.Error("Arch should be populated")
	}
	if s.TraceID == "" {
		t.Error("TraceID should be populated")
	}
	if s.Timestamp == "" {
		t.Error("Timestamp should be populated")
	}
}

// TC-TEL-08: RotateInstallID generates new ID different from old.
func TestRotateInstallID(t *testing.T) {
	cfg := &Config{Enabled: true, InstallID: "old-id"}
	RotateInstallID(cfg)
	if cfg.InstallID == "old-id" {
		t.Error("InstallID should be rotated")
	}
	if cfg.InstallID == "" {
		t.Error("InstallID should not be empty after rotation")
	}
}

// TC-TEL-09: LoadConfig rejects bad JSON.
func TestLoadConfig_BadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.json")
	os.WriteFile(path, []byte("{not json}"), 0o644) //nolint:errcheck
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("want error for bad JSON config")
	}
}

// TC-TEL-10: Emit idempotency — calling twice produces two distinct spans.
func TestEmit_Idempotency(t *testing.T) {
	dir := t.TempDir()
	spanPath := filepath.Join(dir, "t.jsonl")
	cfg := &Config{Enabled: true, InstallID: "id"}
	Emit(spanPath, cfg, Span{Verb: "a"}) //nolint:errcheck
	Emit(spanPath, cfg, Span{Verb: "b"}) //nolint:errcheck
	spans, _ := ReadSpans(spanPath)
	if len(spans) != 2 {
		t.Fatalf("want 2 spans, got %d", len(spans))
	}
	if spans[0].Verb == spans[1].Verb {
		t.Error("spans should have distinct verbs")
	}
}

// TC-TEL-11: JSON keys use snake_case.
func TestEmit_JSONKeys(t *testing.T) {
	dir := t.TempDir()
	spanPath := filepath.Join(dir, "t.jsonl")
	cfg := &Config{Enabled: true, InstallID: "id"}
	Emit(spanPath, cfg, Span{Verb: "ping", DurationMS: 7}) //nolint:errcheck
	raw, _ := os.ReadFile(spanPath)
	var m map[string]any
	json.Unmarshal(raw, &m) //nolint:errcheck
	for _, k := range []string{"trace_id", "span_id", "verb", "exit_code", "duration_ms", "install_id", "os", "arch", "timestamp"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing JSON key %q", k)
		}
	}
}

// TC-TEL-12: data-accuracy — DurationMS is preserved.
func TestEmit_DataAccuracy(t *testing.T) {
	dir := t.TempDir()
	spanPath := filepath.Join(dir, "t.jsonl")
	cfg := &Config{Enabled: true, InstallID: "id"}
	Emit(spanPath, cfg, Span{Verb: "check", DurationMS: 99}) //nolint:errcheck
	spans, _ := ReadSpans(spanPath)
	if spans[0].DurationMS != 99 {
		t.Errorf("DurationMS: want 99, got %d", spans[0].DurationMS)
	}
}

// TC-TEL-13: false-positive guard — disabled telemetry emits nothing even with data.
func TestEmit_FalsePositive_Disabled(t *testing.T) {
	dir := t.TempDir()
	spanPath := filepath.Join(dir, "t.jsonl")
	cfg := &Config{Enabled: false, InstallID: "id"}
	for i := 0; i < 5; i++ {
		Emit(spanPath, cfg, Span{Verb: "scan", DurationMS: int64(i), Timestamp: time.Now().UTC().Format(time.RFC3339)}) //nolint:errcheck
	}
	if _, err := os.Stat(spanPath); !os.IsNotExist(err) {
		t.Error("false-positive: span log created despite telemetry disabled")
	}
}
