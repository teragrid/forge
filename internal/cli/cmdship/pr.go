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

// pr.go — PR creation checkpoint for forge ship.
//
// checkPR uses the gh CLI (https://cli.github.com) to create a draft GitHub
// pull request as the final step of the forge ship pipeline. The PR body is
// assembled from the spec and breakdown files in .forge/specs/<slug>/.
//
// PR creation is always "best-effort": failures degrade to Status="warning"
// so the developer can create the PR manually without the pipeline hard-stopping.
// The checkpoint runs only when --pr is set on the forge ship invocation.
package cmdship

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// checkPR creates a draft GitHub pull request using the gh CLI.
// Returns Status="warning" (never "fail") when gh is absent, unauthenticated,
// or the remote is not a GitHub repository.
func checkPR(root, description string) Checkpoint {
	cp := Checkpoint{Name: "PR"}

	ghPath, err := exec.LookPath("gh")
	if err != nil {
		cp.Status = "warning"
		cp.Detail = "gh CLI not found; install from https://cli.github.com then run: gh pr create"
		return cp
	}

	title := description
	if title == "" {
		title = "forge ship: automated change"
	}

	body := buildPRBody(root, description)
	args := []string{"pr", "create", "--title", title, "--body", body, "--draft"}

	cmd := exec.Command(ghPath, args...) //nolint:gosec // path resolved via LookPath
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		cp.Status = "warning"
		cp.Detail = fmt.Sprintf("gh pr create: %v — %s",
			err, strings.TrimSpace(string(out)))
		return cp
	}

	url := strings.TrimSpace(string(out))
	cp.Status = "ok"
	cp.Detail = fmt.Sprintf("draft PR created: %s", url)
	return cp
}

// buildPRBody assembles the PR description from the spec and breakdown files
// stored in .forge/specs/<slug>/, with a forge ship watermark footer.
func buildPRBody(root, description string) string {
	var sb strings.Builder
	sb.WriteString("## Summary\n")
	if description != "" {
		sb.WriteString(description)
		sb.WriteString("\n\n")
	}
	if description != "" {
		slug := slugify(description)
		specFile := filepath.Join(root, ".forge", "specs", slug, "spec.md")
		if data, err := os.ReadFile(specFile); err == nil {
			sb.WriteString("## Spec\n")
			sb.Write(data)
			sb.WriteString("\n\n")
		}
		breakdownFile := filepath.Join(root, ".forge", "specs", slug, "breakdown.md")
		if data, err := os.ReadFile(breakdownFile); err == nil {
			sb.WriteString("## Breakdown\n")
			sb.Write(data)
			sb.WriteString("\n\n")
		}
		// G-012: include tasks.md checklist and scan summary.
		tasksFile := filepath.Join(root, ".forge", "specs", slug, "tasks.md")
		if data, err := os.ReadFile(tasksFile); err == nil {
			sb.WriteString("## Tasks\n")
			sb.Write(data)
			sb.WriteString("\n\n")
		}
		scanSummaryFile := filepath.Join(root, ".forge", "specs", slug, "scan-summary.md")
		if data, err := os.ReadFile(scanSummaryFile); err == nil {
			sb.WriteString("## Scan Summary\n")
			sb.Write(data)
			sb.WriteString("\n\n")
		}
	}
	sb.WriteString("---\n*Created by `forge ship`*\n")
	return sb.String()
}
