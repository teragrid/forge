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

// Package learningloop implements the opt-in usage share client (M2-09) and
// local aggregator MVP (M2-10).
//
// The learning loop lets teams share anonymised prompt/outcome pairs with
// the Forge community so that the community can improve default prompts,
// detect common failure modes, and publish updated instruction packs.
//
// Privacy guarantees:
//   - Sharing is ALWAYS opt-in. No data leaves the machine unless the user
//     explicitly runs `forge learn share --enable`.
//   - The client strips all user-identifiable information (file paths, project
//     names, git author info) before sending.
//   - The aggregator URL is configurable (default: no-op / localhost stub).
//   - The payload schema is public and versioned.
//
// Error code range: 5800–5899.
package learningloop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/teragrid/forge/internal/errcode"
)

// Error codes.
var (
	ErrShareDisabled   = errcode.Register(errcode.Code(5800), "learning loop share not enabled")
	ErrShareFailed     = errcode.Register(errcode.Code(5801), "learning loop share upload failed")
	ErrAggregateFailed = errcode.Register(errcode.Code(5802), "learning loop aggregation failed")
)

// DefaultAggregatorURL is the endpoint used when no override is configured.
// In v1.0.0 this points to localhost so no data leaves the machine unless the
// user reconfigures it to a community endpoint.
const DefaultAggregatorURL = "http://localhost:7420/api/v1/share"

// Config holds the runtime configuration for the learning loop.
type Config struct {
	Enabled       bool   `json:"enabled"`
	AggregatorURL string `json:"aggregator_url"`
	// MaxBatchSize is the maximum number of events sent per request.
	MaxBatchSize int `json:"max_batch_size"`
}

// DefaultConfig returns a safe-defaults Config (sharing disabled).
func DefaultConfig() Config {
	return Config{
		Enabled:       false,
		AggregatorURL: DefaultAggregatorURL,
		MaxBatchSize:  50,
	}
}

// Event is one anonymised prompt/outcome pair.
type Event struct {
	// ID is a random UUID generated locally; never derived from user data.
	ID string `json:"id"`
	// Verb is the forge verb that produced this event (e.g. "ask", "review").
	Verb string `json:"verb"`
	// Model is the LLM model slug (e.g. "claude-3-5-sonnet-20241022").
	Model string `json:"model"`
	// InputTokens / OutputTokens from the provider response.
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	// Outcome is "success", "error", or "timeout".
	Outcome string `json:"outcome"`
	// ErrorCode, when Outcome=="error", is the FORGE-XXXX error code string.
	ErrorCode string `json:"error_code,omitempty"`
	// Tags are arbitrary key=value labels the caller attaches (no PII).
	Tags map[string]string `json:"tags,omitempty"`
	// RecordedAt is the UTC timestamp the event was captured.
	RecordedAt time.Time `json:"recorded_at"`
}

// Client sends anonymised events to an aggregator endpoint.
type Client struct {
	cfg       Config
	http      *http.Client
	queue     []Event
	queuePath string
}

// NewClient returns a Client configured from cfg. It loads any queued events
// from the on-disk spool in root/.forge/learn-queue.json.
func NewClient(root string, cfg Config) *Client {
	c := &Client{
		cfg:       cfg,
		http:      &http.Client{Timeout: 10 * time.Second},
		queuePath: filepath.Join(root, ".forge", "learn-queue.json"),
	}
	_ = c.loadQueue()
	return c
}

// Record adds an event to the local queue. The queue is flushed automatically
// when it reaches MaxBatchSize, or on the next explicit Flush() call.
func (c *Client) Record(e Event) error {
	if !c.cfg.Enabled {
		return nil // silently discard when not opted-in
	}
	e.RecordedAt = time.Now().UTC()
	c.queue = append(c.queue, e)
	if err := c.saveQueue(); err != nil {
		return err
	}
	if len(c.queue) >= c.cfg.MaxBatchSize {
		return c.Flush(context.Background())
	}
	return nil
}

// Flush sends all queued events to the aggregator and clears the queue.
// It is a no-op when sharing is disabled.
func (c *Client) Flush(ctx context.Context) error {
	if !c.cfg.Enabled {
		return nil
	}
	if len(c.queue) == 0 {
		return nil
	}

	payload, err := json.Marshal(map[string]any{
		"schema_version": "1",
		"events":         c.queue,
	})
	if err != nil {
		return errcode.New(ErrShareFailed, "marshal payload", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.AggregatorURL, bytes.NewReader(payload))
	if err != nil {
		return errcode.New(ErrShareFailed, "create request", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "forge-learning-loop/1")

	resp, err := c.http.Do(req)
	if err != nil {
		return errcode.New(ErrShareFailed, "POST to aggregator", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return errcode.Newf(ErrShareFailed, fmt.Errorf("HTTP %d", resp.StatusCode),
			"aggregator returned error status %d", resp.StatusCode)
	}

	// Clear the queue only on success.
	c.queue = c.queue[:0]
	return c.saveQueue()
}

// QueuedCount returns the number of events waiting to be flushed.
func (c *Client) QueuedCount() int { return len(c.queue) }

func (c *Client) loadQueue() error {
	data, err := os.ReadFile(c.queuePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &c.queue)
}

func (c *Client) saveQueue() error {
	if err := os.MkdirAll(filepath.Dir(c.queuePath), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c.queue, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.queuePath, data, 0o600)
}

// LoadConfig reads the learning loop configuration from
// root/.forge/learn-config.json, returning DefaultConfig() when the file is
// absent or invalid.
func LoadConfig(root string) Config {
	path := filepath.Join(root, ".forge", "learn-config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultConfig()
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig()
	}
	if cfg.AggregatorURL == "" {
		cfg.AggregatorURL = DefaultAggregatorURL
	}
	if cfg.MaxBatchSize == 0 {
		cfg.MaxBatchSize = 50
	}
	return cfg
}

// SaveConfig persists cfg to root/.forge/learn-config.json.
func SaveConfig(root string, cfg Config) error {
	path := filepath.Join(root, ".forge", "learn-config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
