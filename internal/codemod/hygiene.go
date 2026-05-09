package codemod

// hygiene.go — DEV-M0-35: "gitignore" and "gitleaks" codemods.
//
// Both codemods maintain a _managed section_ inside the respective file.
// The user-owned content outside the markers is always preserved.
//
// Drift semantics (TC-35-02):
//   - "drift" means the managed block on disk differs from the canonical
//     version in this binary (i.e. the user hand-edited inside it, or the
//     canonical changed after a forge upgrade).
//   - Without --force: Apply returns ErrManagedBlockDrift; no file is touched.
//   - With --force (ApplyForce): the managed block is overwritten regardless.
//
// ForcedCodemod is the optional interface that codemods implement when they
// support the --force semantic. cmdupgrade checks for it before calling.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrManagedBlockDrift is returned by Apply when the on-disk managed block
// differs from the canonical content and --force was not supplied.
var ErrManagedBlockDrift = errors.New("managed block has drifted from canonical content; rerun with --force to overwrite")

// ForcedCodemod is an optional extension interface for codemods that support
// overwriting a drifted managed block. cmdupgrade detects this interface and
// calls ApplyForce when the user passes --force.
type ForcedCodemod interface {
	Codemod
	ApplyForce(root string, dryRun bool) (Report, error)
}

func init() {
	Default().Register(&gitignoreCM{})
	Default().Register(&gitleaksCM{})
}

// ── gitignore codemod ──────────────────────────────────────────────────────

const (
	giStart = "# forge:gitignore:start"
	giEnd   = "# forge:gitignore:end"
)

// canonicalGitignoreBlock is the full managed block written/expected by forge.
// Version-stamp placeholder allows future bumps without changing file layout.
const canonicalGitignoreBlock = `# forge:gitignore:start
# Managed by forge — do not edit this block. Run "forge upgrade gitignore" to refresh.
# forge-version: managed
.forge/scratch/
.forge/cache/
*.tmp
*.bak
__pycache__/
# forge:gitignore:end
`

type gitignoreCM struct{}

func (gitignoreCM) Name() string { return "gitignore" }
func (gitignoreCM) Description() string {
	return "Update the forge-managed block in .gitignore; preserve user section."
}

// Apply fails with ErrManagedBlockDrift if the on-disk managed block differs
// from the canonical content. Use ApplyForce to override.
func (g gitignoreCM) Apply(root string, dryRun bool) (Report, error) {
	return g.apply(root, dryRun, false)
}

// ApplyForce overwrites the managed block even if it has drifted.
func (g gitignoreCM) ApplyForce(root string, dryRun bool) (Report, error) {
	return g.apply(root, dryRun, true)
}

func (gitignoreCM) apply(root string, dryRun, force bool) (Report, error) {
	path := filepath.Join(root, ".gitignore")
	rep := Report{Codemod: "gitignore", DryRun: dryRun}

	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return rep, fmt.Errorf("read .gitignore: %w", err)
	}
	current := string(raw)

	si := strings.Index(current, giStart)
	ei := strings.Index(current, giEnd)

	if si >= 0 && ei > si {
		// Managed block exists — extract it (inclusive of end marker + newline).
		endIdx := ei + len(giEnd)
		if endIdx < len(current) && current[endIdx] == '\n' {
			endIdx++
		}
		existingBlock := current[si:endIdx]
		if existingBlock == canonicalGitignoreBlock {
			rep.Detail = "no change"
			return rep, nil
		}
		if !force {
			return rep, ErrManagedBlockDrift
		}
		// Force: replace the managed block, keep user content before and after.
		before := current[:si]
		after := current[endIdx:]
		current = before + canonicalGitignoreBlock + after
	} else {
		// No managed block — append it.
		if current != "" && !strings.HasSuffix(current, "\n") {
			current += "\n"
		}
		current += canonicalGitignoreBlock
	}

	rep.Files = []string{".gitignore"}
	rep.Changed = 1
	if !dryRun {
		if err := os.WriteFile(path, []byte(current), 0o600); err != nil {
			return rep, fmt.Errorf("write .gitignore: %w", err)
		}
	}
	return rep, nil
}

// ── gitleaks codemod ───────────────────────────────────────────────────────

const (
	glStart = "# forge:gitleaks:start"
	glEnd   = "# forge:gitleaks:end"
)

const canonicalGitleaksBlock = `# forge:gitleaks:start
# Managed by forge — do not edit this block. Run "forge upgrade gitleaks" to refresh.
title = "forge baseline"

[[rules]]
id = "generic-api-key"
description = "Generic API key"
regex = '''(?i)(api[_-]?key|apikey)[\s:=]+["']?[A-Za-z0-9_\-]{20,}["']?'''

[[rules]]
id = "private-key-block"
description = "PEM private key"
regex = '''-----BEGIN [A-Z ]*PRIVATE KEY-----'''

[[rules]]
id = "openai-key"
description = "OpenAI key"
regex = '''sk-[A-Za-z0-9]{20,}'''

[[rules]]
id = "aws-access-key"
description = "AWS access key"
regex = '''AKIA[0-9A-Z]{16}'''

# forge:gitleaks:end
`

// userSectionHeader is appended after the managed block when a user section
// is created for the first time, providing a clear separation.
const userSectionHeader = `
# ── User-managed rules (add your project-specific rules below) ──────────────
`

type gitleaksCM struct{}

func (gitleaksCM) Name() string { return "gitleaks" }
func (gitleaksCM) Description() string {
	return "Update the forge-managed block in .gitleaks.toml; preserve user rules."
}

func (g gitleaksCM) Apply(root string, dryRun bool) (Report, error) {
	return g.apply(root, dryRun, false)
}

func (g gitleaksCM) ApplyForce(root string, dryRun bool) (Report, error) {
	return g.apply(root, dryRun, true)
}

func (gitleaksCM) apply(root string, dryRun, force bool) (Report, error) {
	path := filepath.Join(root, ".gitleaks.toml")
	rep := Report{Codemod: "gitleaks", DryRun: dryRun}

	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return rep, fmt.Errorf("read .gitleaks.toml: %w", err)
	}
	current := string(raw)

	si := strings.Index(current, glStart)
	ei := strings.Index(current, glEnd)

	switch {
	case si >= 0 && ei > si:
		endIdx := ei + len(glEnd)
		if endIdx < len(current) && current[endIdx] == '\n' {
			endIdx++
		}
		existingBlock := current[si:endIdx]
		if existingBlock == canonicalGitleaksBlock {
			rep.Detail = "no change"
			return rep, nil
		}
		if !force {
			return rep, ErrManagedBlockDrift
		}
		before := current[:si]
		after := current[endIdx:]
		current = before + canonicalGitleaksBlock + after
	case current != "":
		// File exists but has no managed markers — treat whole file as legacy.
		// Wrap it: put the canonical managed block at top, keep existing content
		// as the user section (preserves any custom rules).
		if !force {
			return rep, ErrManagedBlockDrift
		}
		current = canonicalGitleaksBlock + userSectionHeader + current
	default:
		// Brand new file.
		current = canonicalGitleaksBlock + userSectionHeader
	}

	rep.Files = []string{".gitleaks.toml"}
	rep.Changed = 1
	if !dryRun {
		if err := os.WriteFile(path, []byte(current), 0o600); err != nil {
			return rep, fmt.Errorf("write .gitleaks.toml: %w", err)
		}
	}
	return rep, nil
}
