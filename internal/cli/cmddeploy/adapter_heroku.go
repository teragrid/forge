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

// Package cmddeploy — adapter_heroku.go implements the Heroku deploy adapter (M3-09).
//
// Wraps the `heroku container:release` + `heroku container:push` workflow via
// the Heroku CLI subprocess. Requires `heroku` CLI installed and
// `HEROKU_API_KEY` set in the environment (or `heroku login` completed).
//
// Deploy flow:
//
//	heroku container:push web --app <target>
//	heroku container:release web --app <target>
package cmddeploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// HerokuAdapter executes deployments via the Heroku CLI.
type HerokuAdapter struct{}

// Name returns the adapter identifier.
func (a *HerokuAdapter) Name() string { return "heroku" }

// Deploy runs `heroku container:push web && heroku container:release web` for cfg.Target app.
func (a *HerokuAdapter) Deploy(ctx context.Context, cfg DeployConfig, tag string, dryRun bool) (string, error) {
	if err := requireTool("heroku"); err != nil {
		return "", fmt.Errorf("heroku adapter: %w", err)
	}
	app := cfg.Target
	if app == "" {
		return "", fmt.Errorf("heroku adapter: no app name configured (set DeployConfig.Target to the Heroku app name)")
	}

	pushArgs := []string{"container:push", "web", "--app", app}
	releaseArgs := []string{"container:release", "web", "--app", app}
	if tag != "" && tag != "HEAD" {
		pushArgs = append(pushArgs, "--arg", "IMAGE_TAG="+tag)
	}

	if dryRun {
		return fmt.Sprintf("heroku adapter (dry-run): would run: heroku %v && heroku %v", pushArgs, releaseArgs), nil
	}

	// Push
	pushCmd := exec.CommandContext(ctx, "heroku", pushArgs...)
	pushCmd.Env = os.Environ()
	if out, err := pushCmd.CombinedOutput(); err != nil {
		return string(out), fmt.Errorf("heroku container:push: %w\n%s", err, string(out))
	}

	// Release
	releaseCmd := exec.CommandContext(ctx, "heroku", releaseArgs...)
	releaseCmd.Env = os.Environ()
	out, err := releaseCmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("heroku container:release: %w\n%s", err, string(out))
	}
	return string(out), nil
}

// Rollback runs `heroku releases:rollback --app <target>`.
func (a *HerokuAdapter) Rollback(ctx context.Context, cfg DeployConfig, to string, dryRun bool) (string, error) {
	if err := requireTool("heroku"); err != nil {
		return "", fmt.Errorf("heroku adapter: %w", err)
	}
	app := cfg.Target
	if app == "" {
		return "", fmt.Errorf("heroku adapter: no app name configured")
	}
	args := []string{"releases:rollback", "--app", app}
	if to != "" {
		args = append(args, to)
	}
	if dryRun {
		return fmt.Sprintf("heroku adapter (dry-run): would run: heroku %v", args), nil
	}
	cmd := exec.CommandContext(ctx, "heroku", args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("heroku releases:rollback: %w\n%s", err, string(out))
	}
	return string(out), nil
}
