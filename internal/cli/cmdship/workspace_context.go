// Copyright 2024 The Forge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// workspace_context.go — G9: workspace/project context collection for spec checkpoint.
//
// collectWorkspaceContext performs a deterministic, zero-LLM-call scan of the
// project root and produces a structured snapshot written to
// .forge/specs/<slug>/workspace-context.md.
//
// The snapshot is injected into spec generation and review prompts so that the
// LLM has concrete knowledge of the existing system (tech stack, conventions,
// recent activity, existing features) before writing a new spec.
//
// Output is capped at ~600 tokens to orient the LLM without bloating context.
package cmdship

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// WorkspaceContextResult holds the collected snapshot.
type WorkspaceContextResult struct {
	// SnapshotPath is the path to the written workspace-context.md file.
	// Empty when the .forge/specs/<slug>/ directory is not writable.
	SnapshotPath string

	// Content is the markdown snapshot content. Always populated even when
	// SnapshotPath is empty (write failure does not block spec generation).
	Content string

	// TechStack is the list of detected technology labels (e.g. "Go module").
	TechStack []string

	// HasGit is true when a git repository was detected and log was retrieved.
	HasGit bool
}

// techStackIndicator maps a filename (relative to root) to a human-readable label.
// Order of evaluation does not matter — all present files are reported.
var techStackIndicators = map[string]string{
	"go.mod":              "Go module",
	"go.work":             "Go workspace (monorepo)",
	"package.json":        "Node.js",
	"requirements.txt":    "Python (requirements.txt)",
	"pyproject.toml":      "Python (pyproject.toml)",
	"Cargo.toml":          "Rust",
	"pom.xml":             "Java (Maven)",
	"build.gradle":        "Java (Gradle)",
	"build.gradle.kts":    "Java (Gradle Kotlin DSL)",
	"Gemfile":             "Ruby",
	"composer.json":       "PHP",
	"mix.exs":             "Elixir",
	"Makefile":            "Make",
	"Dockerfile":          "Docker",
	"docker-compose.yml":  "Docker Compose",
	"docker-compose.yaml": "Docker Compose",
}

// collectWorkspaceContext scans the project root and writes workspace-context.md
// to .forge/specs/<slug>/. This is a deterministic step — no LLM calls are made.
func collectWorkspaceContext(root, slug string) WorkspaceContextResult {
	var res WorkspaceContextResult
	var sb strings.Builder

	sb.WriteString("# Workspace Context Snapshot\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339)))

	// 1. Tech stack detection.
	stack := detectTechStack(root)
	res.TechStack = stack
	if len(stack) > 0 {
		sb.WriteString("## Tech Stack\n")
		for _, t := range stack {
			// For Go modules, include module path and version for extra precision.
			if t == "Go module" {
				if mod := readGoModSummary(root); mod != "" {
					t = "Go module (" + mod + ")"
				}
			}
			sb.WriteString(fmt.Sprintf("- %s\n", t))
		}
		sb.WriteString("\n")
	}

	// 2. Top-level project structure (non-hidden directories only).
	if dirs := scanTopLevelDirs(root); len(dirs) > 0 {
		sb.WriteString("## Project Structure\n")
		sb.WriteString("- " + strings.Join(dirs, "/, ") + "/\n\n")
	}

	// 3. Recent git activity (last 10 commits).
	if log := recentGitLog(root, 10); log != "" {
		res.HasGit = true
		sb.WriteString("## Recent Changes (last 10 commits)\n```\n")
		sb.WriteString(log)
		sb.WriteString("\n```\n\n")
	}

	// 4. Existing feature specs — helps LLM avoid duplicating existing work.
	if specs := listExistingSpecs(root); len(specs) > 0 {
		sb.WriteString("## Existing Feature Specs (avoid duplicates)\n")
		sb.WriteString("- " + strings.Join(specs, ", ") + "\n\n")
	}

	// 5. Project conventions from AGENTS.md or .github/copilot-instructions.md.
	if conv := loadConventionSummary(root); conv != "" {
		sb.WriteString("## Project Conventions\n")
		sb.WriteString(conv)
		sb.WriteString("\n")
	}

	res.Content = sb.String()

	// Write to .forge/specs/<slug>/workspace-context.md.
	contextFile := filepath.Join(root, ".forge", "specs", slug, "workspace-context.md")
	if err := os.MkdirAll(filepath.Dir(contextFile), 0o755); err == nil {
		if writeErr := os.WriteFile(contextFile, []byte(res.Content), 0o600); writeErr == nil {
			res.SnapshotPath = contextFile
		}
	}

	return res
}

// detectTechStack returns sorted labels for every tech-stack indicator file found.
func detectTechStack(root string) []string {
	var found []string
	for file, label := range techStackIndicators {
		if _, err := os.Stat(filepath.Join(root, file)); err == nil {
			found = append(found, label)
		}
	}
	// Also detect .github/ directory (GitHub Actions).
	if _, err := os.Stat(filepath.Join(root, ".github")); err == nil {
		found = append(found, "GitHub Actions CI")
	}
	sort.Strings(found)
	return found
}

// readGoModSummary returns a short "module/path go X.Y" string from go.mod,
// or empty string when go.mod is absent or unparseable.
func readGoModSummary(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	var modPath, goVer string
	for _, line := range strings.SplitN(string(data), "\n", 20) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") && modPath == "" {
			modPath = strings.TrimPrefix(line, "module ")
		}
		if strings.HasPrefix(line, "go ") && goVer == "" {
			goVer = strings.TrimPrefix(line, "go ")
		}
		if modPath != "" && goVer != "" {
			break
		}
	}
	if modPath == "" {
		return ""
	}
	if goVer != "" {
		return fmt.Sprintf("%s, go %s", modPath, goVer)
	}
	return modPath
}

// scanTopLevelDirs returns the names of non-hidden top-level directories in root.
func scanTopLevelDirs(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue // skip .git, .forge, etc.
		}
		dirs = append(dirs, name)
	}
	return dirs
}

// recentGitLog runs `git log --oneline -n <n>` and returns the trimmed output.
// Returns empty string when git is unavailable or the directory is not a repo.
func recentGitLog(root string, n int) string {
	cmd := exec.Command("git", "-C", root, "log", "--oneline", fmt.Sprintf("-n%d", n)) //nolint:gosec // G204: root is validated via os.Stat before reaching here
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// listExistingSpecs returns the slugs of feature specs already in .forge/specs/.
// The current slug is excluded (it is the feature being planned).
func listExistingSpecs(root string) []string {
	specsDir := filepath.Join(root, ".forge", "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return nil
	}
	var specs []string
	for _, e := range entries {
		if e.IsDir() {
			specs = append(specs, e.Name())
		}
	}
	return specs
}

// loadConventionSummary reads the most relevant conventions file and returns
// a summary capped at 500 characters so it doesn't overwhelm the spec prompt.
// Files tried in order: AGENTS.md, .github/copilot-instructions.md, CONTRIBUTING.md.
func loadConventionSummary(root string) string {
	candidates := []struct {
		path  string
		label string
	}{
		{filepath.Join(root, "AGENTS.md"), "AGENTS.md"},
		{filepath.Join(root, ".github", "copilot-instructions.md"), "copilot-instructions.md"},
		{filepath.Join(root, "CONTRIBUTING.md"), "CONTRIBUTING.md"},
	}
	const maxChars = 500
	for _, c := range candidates {
		data, err := os.ReadFile(c.path)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		// Strip markdown headings and blank lines for density.
		var lines []string
		for _, line := range strings.Split(content, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			lines = append(lines, trimmed)
		}
		condensed := strings.Join(lines, " ")
		if len(condensed) > maxChars {
			condensed = condensed[:maxChars] + " [truncated]"
		}
		return fmt.Sprintf("(from %s) %s\n", c.label, condensed)
	}
	return ""
}
