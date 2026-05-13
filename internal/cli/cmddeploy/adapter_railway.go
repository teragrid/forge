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

// Package cmddeploy — adapter_railway.go implements the Railway deploy adapter (M2-06).
//
// Wraps the `railway up` CLI subprocess. Requires the Railway CLI to be
// installed and `RAILWAY_TOKEN` set in the environment.
package cmddeploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// RailwayAdapter executes deployments via the Railway CLI.
type RailwayAdapter struct{}

// Name returns the adapter identifier.
func (a *RailwayAdapter) Name() string { return "railway" }

// Deploy runs `railway up [--detach]` in the given working directory.
func (a *RailwayAdapter) Deploy(ctx context.Context, _ DeployConfig, tag string, dryRun bool) (string, error) {
	if err := requireTool("railway"); err != nil {
		return "", fmt.Errorf("railway adapter: %w", err)
	}
	args := []string{"up", "--detach"}
	if tag != "" {
		// Railway doesn't support image tags directly; pass as env var override
		args = append(args, "--service", tag)
	}
	if dryRun {
		return fmt.Sprintf("railway adapter (dry-run): would run: railway %v", args), nil
	}
	cmd := exec.CommandContext(ctx, "railway", args...)
	cmd.Env = os.Environ() // inherits RAILWAY_TOKEN
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("railway up: %w\n%s", err, string(out))
	}
	return string(out), nil
}

// Rollback runs `railway rollback [--deployment <to>]`.
func (a *RailwayAdapter) Rollback(ctx context.Context, _ DeployConfig, to string, dryRun bool) (string, error) {
	if err := requireTool("railway"); err != nil {
		return "", fmt.Errorf("railway adapter: %w", err)
	}
	args := []string{"rollback"}
	if to != "" {
		args = append(args, "--deployment", to)
	}
	if dryRun {
		return fmt.Sprintf("railway adapter (dry-run): would run: railway %v", args), nil
	}
	cmd := exec.CommandContext(ctx, "railway", args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("railway rollback: %w\n%s", err, string(out))
	}
	return string(out), nil
}
