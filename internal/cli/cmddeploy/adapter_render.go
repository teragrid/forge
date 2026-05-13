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

// Package cmddeploy — adapter_render.go implements the Render deploy adapter (M3-09).
//
// Render supports deploy hooks (a URL that triggers a redeploy) and optionally
// the Render CLI. This adapter uses the deploy-hook URL stored in
// DeployConfig.Target (set via `forge deploy configure --adapter render
// --target <hook-url>`), falling back to the RENDER_DEPLOY_HOOK_URL env var.
package cmddeploy

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

// RenderAdapter triggers deployments via Render deploy hooks.
type RenderAdapter struct{}

// Name returns the adapter identifier.
func (a *RenderAdapter) Name() string { return "render" }

// Deploy POSTs to the Render deploy-hook URL to trigger a redeploy.
// The hook URL comes from cfg.Target or the RENDER_DEPLOY_HOOK_URL env var.
func (a *RenderAdapter) Deploy(ctx context.Context, cfg DeployConfig, _ string, dryRun bool) (string, error) {
	hookURL := cfg.Target
	if hookURL == "" {
		hookURL = os.Getenv("RENDER_DEPLOY_HOOK_URL")
	}
	if hookURL == "" {
		return "", fmt.Errorf("render adapter: no deploy hook URL configured (set DeployConfig.Target or RENDER_DEPLOY_HOOK_URL)")
	}

	// Validate URL to prevent SSRF (defence-in-depth).
	parsed, err := url.ParseRequestURI(hookURL)
	if err != nil {
		return "", fmt.Errorf("render adapter: invalid hook URL: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("render adapter: hook URL scheme %q not allowed", parsed.Scheme)
	}

	if dryRun {
		return fmt.Sprintf("render adapter (dry-run): would POST to %s", hookURL), nil
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hookURL, bytes.NewReader(nil))
	if err != nil {
		return "", fmt.Errorf("render adapter: build request: %w", err)
	}
	req.Header.Set("User-Agent", "forge/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("render adapter: POST hook: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("render adapter: hook returned HTTP %d", resp.StatusCode)
	}
	return fmt.Sprintf("render adapter: deploy triggered via hook (HTTP %d)", resp.StatusCode), nil
}

// Rollback is not directly supported by Render deploy hooks.
// Use the Render dashboard or API to revert to a previous deploy.
func (a *RenderAdapter) Rollback(_ context.Context, cfg DeployConfig, to string, dryRun bool) (string, error) {
	if dryRun {
		return fmt.Sprintf("render adapter (dry-run): rollback to %q requires Render dashboard/API", to), nil
	}
	return "", fmt.Errorf("render adapter: rollback must be performed via the Render dashboard or API; target=%s to=%s", cfg.Target, to)
}
