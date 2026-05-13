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

// Package cmddeploy — adapter_fly.go implements the Fly.io deploy adapter (M2-05).
//
// Wraps the `flyctl deploy` CLI subprocess. Requires `flyctl` to be installed
// and `FLY_API_TOKEN` set in the environment (or `fly auth login` completed).
package cmddeploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// FlyAdapter executes deployments via flyctl.
type FlyAdapter struct{}

// Name returns the adapter identifier.
func (a *FlyAdapter) Name() string { return "fly" }

// Deploy runs `flyctl deploy [--image <tag>]` in the given working directory.
func (a *FlyAdapter) Deploy(ctx context.Context, _ DeployConfig, tag string, dryRun bool) (string, error) {
	if err := requireTool("flyctl"); err != nil {
		return "", fmt.Errorf("fly adapter: %w", err)
	}
	args := []string{"deploy"}
	if tag != "" {
		args = append(args, "--image", tag)
	}
	if dryRun {
		return fmt.Sprintf("fly adapter (dry-run): would run: flyctl %v", args), nil
	}
	cmd := exec.CommandContext(ctx, "flyctl", args...)
	// Use current working directory; callers chdir before invoking
	cmd.Env = os.Environ() // inherit env (FLY_API_TOKEN etc.)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("flyctl deploy: %w\n%s", err, string(out))
	}
	return string(out), nil
}

// Rollback runs `flyctl releases rollback [--version <tag>]`.
func (a *FlyAdapter) Rollback(ctx context.Context, _ DeployConfig, to string, dryRun bool) (string, error) {
	if err := requireTool("flyctl"); err != nil {
		return "", fmt.Errorf("fly adapter: %w", err)
	}
	args := []string{"releases", "rollback"}
	if to != "" {
		args = append(args, "--version", to)
	}
	if dryRun {
		return fmt.Sprintf("fly adapter (dry-run): would run: flyctl %v", args), nil
	}
	cmd := exec.CommandContext(ctx, "flyctl", args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("flyctl rollback: %w\n%s", err, string(out))
	}
	return string(out), nil
}

func requireTool(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("tool %q not found in PATH: install it first", name)
	}
	return nil
}
