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

// Package incident – webhook.go implements the ADR-021 status-page webhook
// integration (M2-25). When an incident is created or its state changes, a
// JSON payload is POST-ed to the configured webhook URL so the external status
// page reflects the current incident state automatically.
package incident

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

// WebhookConfig holds the status-page webhook settings.
// The URL and optional HMAC secret come from .forge/config.toml or
// the FORGE_WEBHOOK_URL / FORGE_WEBHOOK_SECRET environment variables.
type WebhookConfig struct {
	// URL is the webhook endpoint (e.g. https://hooks.instatus.com/service/<token>).
	// Leave empty to disable webhook delivery.
	URL string
	// Secret is an optional shared secret used to sign the payload
	// (HMAC-SHA256 over the raw body, sent as X-Forge-Signature header).
	// Injected via FORGE_WEBHOOK_SECRET env var; never stored on disk.
	Secret string
}

// DefaultWebhookConfig reads FORGE_WEBHOOK_URL and FORGE_WEBHOOK_SECRET from
// the environment. Returns an empty (disabled) config if neither is set.
func DefaultWebhookConfig() WebhookConfig {
	return WebhookConfig{
		URL:    os.Getenv("FORGE_WEBHOOK_URL"),
		Secret: os.Getenv("FORGE_WEBHOOK_SECRET"),
	}
}

// webhookPayload is the canonical JSON shape sent to the status page.
type webhookPayload struct {
	SchemaVersion string   `json:"schema_version"`
	EventType     string   `json:"event_type"` // "incident.created" | "incident.updated" | "incident.resolved"
	IncidentID    string   `json:"incident_id"`
	Title         string   `json:"title"`
	State         State    `json:"state"`
	Severity      Severity `json:"severity"`
	Systems       []string `json:"systems"`
	OccurredAt    string   `json:"occurred_at"` // RFC3339
}

// Notifier sends webhook notifications to the configured status page.
type Notifier struct {
	cfg    WebhookConfig
	client *http.Client
}

// NewNotifier creates a Notifier with the given config.
// Callers should obtain the config via DefaultWebhookConfig() or from
// their own config loader.
func NewNotifier(cfg WebhookConfig) *Notifier {
	return &Notifier{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// eventType derives the webhook event type from the incident state.
func eventType(state State) string {
	switch state {
	case StateIdentified, StateInvestigating:
		return "incident.created"
	case StateFixed, StatePostMortemPublished:
		return "incident.resolved"
	default:
		return "incident.updated"
	}
}

// Notify sends a webhook notification for the given incident.
// It is a no-op when cfg.URL is empty. Errors are returned but callers
// should treat them as warnings — a failing webhook must not block the CLI.
func (n *Notifier) Notify(ctx context.Context, inc *Incident) error {
	if n.cfg.URL == "" {
		return nil // webhook disabled
	}

	// Validate URL (defence-in-depth: reject javascript:/file:/ etc.)
	parsed, err := url.ParseRequestURI(n.cfg.URL)
	if err != nil {
		return fmt.Errorf("webhook: invalid URL %q: %w", n.cfg.URL, err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("webhook: URL scheme %q is not allowed (use https)", parsed.Scheme)
	}

	payload := webhookPayload{
		SchemaVersion: "1",
		EventType:     eventType(inc.State),
		IncidentID:    inc.ID,
		Title:         inc.Title,
		State:         inc.State,
		Severity:      inc.Severity,
		Systems:       inc.Systems,
		OccurredAt:    inc.UpdatedAt.Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhook: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "forge/1.0")

	// HMAC-SHA256 signature when a secret is configured.
	// The signature is computed over the raw JSON body.
	if n.cfg.Secret != "" {
		sig, err := hmacSHA256Hex([]byte(n.cfg.Secret), body)
		if err != nil {
			return fmt.Errorf("webhook: sign: %w", err)
		}
		req.Header.Set("X-Forge-Signature", "sha256="+sig)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: POST %s: %w", n.cfg.URL, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook: server returned %d for incident %s", resp.StatusCode, inc.ID)
	}
	return nil
}
