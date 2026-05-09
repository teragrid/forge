package codemod

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func init() {
	Default().Register(&dependabotBaseline{})
	Default().Register(&preCommitBaseline{})
}

// ----- dependabot-baseline -----

type dependabotBaseline struct{}

func (dependabotBaseline) Name() string { return "dependabot-baseline" }
func (dependabotBaseline) Description() string {
	return "Create .github/dependabot.yml with weekly updates for detected ecosystems."
}

// detectEcosystems scans root for ecosystem marker files. The set is sorted
// alphabetically for deterministic output.
func detectEcosystems(root string) []string {
	candidates := map[string]string{
		"gomod":          "go.mod",
		"npm":            "package.json",
		"pip-req":        "requirements.txt",
		"pip-pyproject":  "pyproject.toml",
		"docker":         "Dockerfile",
		"github-actions": ".github/workflows",
	}
	seen := map[string]bool{}
	for key, marker := range candidates {
		if _, err := os.Stat(filepath.Join(root, marker)); err == nil {
			// Both pip markers map to the same dependabot ecosystem.
			eco := key
			switch key {
			case "pip-req", "pip-pyproject":
				eco = "pip"
			}
			seen[eco] = true
		}
	}
	out := make([]string, 0, len(seen))
	for e := range seen {
		out = append(out, e)
	}
	sort.Strings(out)
	return out
}

// renderDependabot builds the YAML body. Hand-rolled (no yaml dep).
func renderDependabot(ecosystems []string) string {
	var buf bytes.Buffer
	buf.WriteString("# Managed by forge — baseline dependabot config. Add custom rules below.\n")
	buf.WriteString("version: 2\n")
	buf.WriteString("updates:\n")
	if len(ecosystems) == 0 {
		// Always include github-actions as a safe default so the file is non-empty.
		ecosystems = []string{"github-actions"}
	}
	for _, eco := range ecosystems {
		dir := "/"
		if eco == "github-actions" {
			dir = "/"
		}
		fmt.Fprintf(&buf, "  - package-ecosystem: %q\n", eco)
		fmt.Fprintf(&buf, "    directory: %q\n", dir)
		buf.WriteString("    schedule:\n")
		buf.WriteString("      interval: \"weekly\"\n")
		buf.WriteString("    open-pull-requests-limit: 10\n")
	}
	return buf.String()
}

func (d dependabotBaseline) Apply(root string, dryRun bool) (Report, error) {
	rel := filepath.Join(".github", "dependabot.yml")
	path := filepath.Join(root, rel)
	rep := Report{Codemod: d.Name(), DryRun: dryRun}

	if _, err := os.Stat(path); err == nil {
		rep.Detail = "already present"
		return rep, nil
	}
	body := renderDependabot(detectEcosystems(root))
	rep.Files = []string{rel}
	rep.Changed = 1
	if !dryRun {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return rep, fmt.Errorf("mkdir .github: %w", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			return rep, fmt.Errorf("write dependabot.yml: %w", err)
		}
	}
	return rep, nil
}

// ----- pre-commit-baseline -----

type preCommitBaseline struct{}

func (preCommitBaseline) Name() string { return "pre-commit-baseline" }
func (preCommitBaseline) Description() string {
	return "Create .pre-commit-config.yaml with baseline hygiene + secret-scan hooks."
}

const preCommitBaseHooks = `# Managed by forge — baseline pre-commit hooks. Add custom hooks below.
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.6.0
    hooks:
      - id: trailing-whitespace
      - id: end-of-file-fixer
      - id: check-yaml
      - id: check-added-large-files
        args: ["--maxkb=512"]
  - repo: https://github.com/gitleaks/gitleaks
    rev: v8.18.4
    hooks:
      - id: gitleaks
`

const preCommitGoHook = `  - repo: https://github.com/dnephin/pre-commit-golang
    rev: v0.5.1
    hooks:
      - id: go-fmt
      - id: go-vet-mod
`

func (p preCommitBaseline) Apply(root string, dryRun bool) (Report, error) {
	rel := ".pre-commit-config.yaml"
	path := filepath.Join(root, rel)
	rep := Report{Codemod: p.Name(), DryRun: dryRun}

	if _, err := os.Stat(path); err == nil {
		rep.Detail = "already present"
		return rep, nil
	}
	body := preCommitBaseHooks
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		body += preCommitGoHook
	}
	rep.Files = []string{rel}
	rep.Changed = 1
	if !dryRun {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			return rep, fmt.Errorf("write .pre-commit-config.yaml: %w", err)
		}
	}
	return rep, nil
}
