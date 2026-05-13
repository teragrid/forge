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

// Package cmddeploy — adapter_awsecs.go implements the AWS ECS deploy adapter (M3-09).
//
// Wraps the AWS CLI `ecs update-service --force-new-deployment` subprocess.
// Requires `aws` CLI installed and AWS credentials configured (via
// AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY / AWS_PROFILE or instance role).
//
// DeployConfig fields:
//
//	Target  — ECS service ARN or "<cluster>/<service>" shorthand
//	Env     — optional: {"AWS_REGION": "us-east-1"}
//
// Deploy flow:
//
//	aws ecs update-service --cluster <cluster> --service <service> --force-new-deployment
package cmddeploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// AWSECSAdapter executes deployments via the AWS CLI targeting ECS services.
type AWSECSAdapter struct{}

// Name returns the adapter identifier.
func (a *AWSECSAdapter) Name() string { return "aws-ecs" }

// Deploy triggers a new ECS deployment via `aws ecs update-service`.
// cfg.Target must be in the form "cluster-name/service-name".
func (a *AWSECSAdapter) Deploy(ctx context.Context, cfg DeployConfig, tag string, dryRun bool) (string, error) {
	if err := requireTool("aws"); err != nil {
		return "", fmt.Errorf("aws-ecs adapter: %w", err)
	}
	cluster, service, err := parseECSTarget(cfg.Target)
	if err != nil {
		return "", fmt.Errorf("aws-ecs adapter: %w", err)
	}

	args := []string{
		"ecs", "update-service",
		"--cluster", cluster,
		"--service", service,
		"--force-new-deployment",
	}
	// Optionally set task definition image tag via environment variable.
	// Full image override requires a more complex task-definition update;
	// the tag is documented for callers using external CI pipelines.
	_ = tag // tag is informational; ECS picks up the image registered in the task def

	if dryRun {
		return fmt.Sprintf("aws-ecs adapter (dry-run): would run: aws %v", args), nil
	}

	env := buildEnv(cfg.Env)
	cmd := exec.CommandContext(ctx, "aws", args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("aws ecs update-service: %w\n%s", err, string(out))
	}
	return string(out), nil
}

// Rollback stops the current ECS deployment and forces a re-deploy of the previous task definition.
// With ECS, rollback is achieved by updating the service to the previous task-def revision.
func (a *AWSECSAdapter) Rollback(ctx context.Context, cfg DeployConfig, to string, dryRun bool) (string, error) {
	if err := requireTool("aws"); err != nil {
		return "", fmt.Errorf("aws-ecs adapter: %w", err)
	}
	cluster, service, err := parseECSTarget(cfg.Target)
	if err != nil {
		return "", fmt.Errorf("aws-ecs adapter: %w", err)
	}

	if to == "" {
		return "", fmt.Errorf("aws-ecs adapter: rollback requires --to <task-definition-revision>")
	}

	args := []string{
		"ecs", "update-service",
		"--cluster", cluster,
		"--service", service,
		"--task-definition", to,
	}
	if dryRun {
		return fmt.Sprintf("aws-ecs adapter (dry-run): would run: aws %v", args), nil
	}

	env := buildEnv(cfg.Env)
	cmd := exec.CommandContext(ctx, "aws", args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("aws ecs update-service (rollback): %w\n%s", err, string(out))
	}
	return string(out), nil
}

// parseECSTarget splits "cluster/service" into its components.
func parseECSTarget(target string) (cluster, service string, err error) {
	if target == "" {
		return "", "", fmt.Errorf("no ECS target configured (set DeployConfig.Target to \"cluster/service\")")
	}
	parts := strings.SplitN(target, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid ECS target %q: expected \"cluster/service\"", target)
	}
	return parts[0], parts[1], nil
}

// buildEnv merges the current process environment with adapter-specific env vars.
func buildEnv(extra map[string]string) []string {
	base := os.Environ()
	for k, v := range extra {
		base = append(base, k+"="+v)
	}
	return base
}
