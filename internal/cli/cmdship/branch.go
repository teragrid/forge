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

// branch.go — feature-branch management for forge ship.
//
// When the user runs "forge ship <feature>" from a protected branch (main,
// master, develop, dev, trunk) the pipeline automatically creates and checks
// out a feature/<slug> branch so that all generated artefacts (specs, tests,
// code) are kept off the default branch.
//
// The operation is best-effort: if git is unavailable or the branch already
// exists the warning is printed but the pipeline is never stopped.
package cmdship

import (
	"fmt"
	"os/exec"
	"strings"
)

// protectedBranches are the branch names on which "forge ship" refuses to
// commit directly. Any other branch name is treated as an already-created
// feature branch and left untouched.
var protectedBranches = map[string]bool{
	"main":       true,
	"master":     true,
	"develop":    true,
	"dev":        true,
	"trunk":      true,
	"production": true,
	"prod":       true,
}

// featureBranchResult describes what happened during ensureFeatureBranch.
type featureBranchResult struct {
	// Branch is the name of the branch we are (or will be) working on.
	Branch string
	// Created is true when a new branch was created by this call.
	Created bool
	// Warning is non-empty when the operation degraded gracefully.
	Warning string
}

// ensureFeatureBranch ensures the working tree is on a feature branch before
// the ship pipeline starts.
//
//   - If the current branch is already a non-protected branch (e.g. a branch
//     the user created manually), we simply return it as-is.
//   - If the current branch is a protected branch, we create and check out
//     feature/<slug>.
//   - If git is unavailable or any git command fails, a warning is returned
//     and the pipeline continues on the current branch.
func ensureFeatureBranch(root, slug string) featureBranchResult {
	current, err := currentBranchName(root)
	if err != nil {
		return featureBranchResult{
			Branch:  "",
			Warning: fmt.Sprintf("could not determine current branch — %v; proceeding on current branch", err),
		}
	}

	// Already on a feature or topic branch — no action needed.
	if !protectedBranches[current] {
		return featureBranchResult{Branch: current}
	}

	// On a protected branch — create feature/<slug>.
	branchName := "feature/" + slug

	gitPath, err := exec.LookPath("git")
	if err != nil {
		return featureBranchResult{
			Branch:  current,
			Warning: "git not found in PATH; proceeding on " + current,
		}
	}

	cmd := exec.Command(gitPath, "checkout", "-b", branchName) //nolint:gosec // git path from LookPath
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Branch may already exist — attempt to switch to it.
		switchCmd := exec.Command(gitPath, "checkout", branchName) //nolint:gosec
		switchCmd.Dir = root
		switchOut, switchErr := switchCmd.CombinedOutput()
		if switchErr != nil {
			return featureBranchResult{
				Branch: current,
				Warning: fmt.Sprintf("could not create or switch to branch %q: %v — %s; proceeding on %s",
					branchName, err, strings.TrimSpace(string(out)), current),
			}
		}
		_ = switchOut
		return featureBranchResult{Branch: branchName, Created: false}
	}

	return featureBranchResult{Branch: branchName, Created: true}
}

// currentBranchName returns the name of the currently checked-out branch.
// Returns an error when git is unavailable or the directory is not a repo.
func currentBranchName(root string) (string, error) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git not found in PATH")
	}
	cmd := exec.Command(gitPath, "rev-parse", "--abbrev-ref", "HEAD") //nolint:gosec
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w — %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
