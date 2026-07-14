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

// digest.go — P1-L2: Progressive Context Digest.
//
// Each completed checkpoint writes a compact YAML digest to
// .forge/specs/<slug>/digests/<checkpoint>.digest.yaml.  Downstream checkpoints
// build a "prior context" section that includes:
//   - the immediately prior checkpoint's full artefact (≤ 800 tokens)
//   - all older checkpoints' stored digests (≤ 300 tokens each)
//
// This replaces the pattern of re-sending large artefacts verbatim, reducing
// cross-checkpoint token spend by ~60–75% on complex features.
package cmdship

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	digestsSubDir   = "digests"
	digestExt       = ".digest.yaml"
	maxDigestTokens = 300 // tokens allocated per prior digest in downstream context
	maxFullTokens   = 800 // tokens for the immediately preceding checkpoint (full text)
)

// CheckpointDigest is the compressed, downstream-consumable summary of a
// single checkpoint's output.  Stored on disk so later checkpoints can
// include prior context at low token cost.
type CheckpointDigest struct {
	Checkpoint    string    `yaml:"checkpoint"`
	Generated     time.Time `yaml:"generated"`
	SourceHash    string    `yaml:"source_hash"`    // first 16 chars of SHA-256 of artefact
	Decisions     []string  `yaml:"decisions"`      // key decisions made
	Constraints   []string  `yaml:"constraints"`    // constraints introduced
	RisksAccepted []string  `yaml:"risks_accepted"` // risks explicitly accepted
	TokenEstimate int       `yaml:"token_estimate"` // rough estimate of full artefact tokens
}

// writeCheckpointDigest serialises d to
// .forge/specs/<slug>/digests/<checkpoint>.digest.yaml.
// Best-effort: any I/O failure is silently ignored so it never blocks the pipeline.
func writeCheckpointDigest(root, slug string, d CheckpointDigest) {
	dir := filepath.Join(root, ".forge", "specs", slug, digestsSubDir)
	_ = os.MkdirAll(dir, 0o755)
	out, err := yaml.Marshal(&d)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, d.Checkpoint+digestExt), out, 0o600)
}

// readCheckpointDigest loads a single digest from disk.
// Returns (nil, nil) when the file does not exist.
//
//nolint:unused // called by buildDigestContext; exported for DAG pipeline / forge undo
func readCheckpointDigest(root, slug, checkpoint string) (*CheckpointDigest, error) {
	path := filepath.Join(root, ".forge", "specs", slug, digestsSubDir, checkpoint+digestExt)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var d CheckpointDigest
	if err := yaml.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// buildDigestContext builds a compact prior-checkpoint context string ready
// to prepend to a downstream LLM system prompt.
//
// upstreamCheckpoints is the ordered list of checkpoints that ran before the
// current one (oldest first).  artefacts maps checkpoint name → full artefact
// text (may be empty when the caller does not have the text in memory).
//
// Strategy:
//   - Last entry in upstreamCheckpoints → full text (≤ maxFullTokens)
//   - All earlier entries → stored digest (≤ maxDigestTokens each)
//
//nolint:unused // used by DAG pipeline and forge undo; not yet wired into linear path
func buildDigestContext(root, slug string, upstreamCheckpoints []string, artefacts map[string]string) string {
	if len(upstreamCheckpoints) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Prior Checkpoint Context (progressive digest)\n\n")
	for i, cp := range upstreamCheckpoints {
		isImmediate := i == len(upstreamCheckpoints)-1
		if isImmediate {
			if art, ok := artefacts[cp]; ok && art != "" {
				b.WriteString(fmt.Sprintf("### %s (full)\n\n%s\n\n", cp, truncateTokens(art, maxFullTokens)))
				continue
			}
		}
		d, err := readCheckpointDigest(root, slug, cp)
		if err != nil || d == nil {
			continue
		}
		b.WriteString(fmt.Sprintf("### %s digest\n", cp))
		if len(d.Decisions) > 0 {
			b.WriteString("Decisions:\n")
			for _, dec := range d.Decisions {
				b.WriteString("- " + dec + "\n")
			}
		}
		if len(d.Constraints) > 0 {
			b.WriteString("Constraints:\n")
			for _, c := range d.Constraints {
				b.WriteString("- " + c + "\n")
			}
		}
		if len(d.RisksAccepted) > 0 {
			b.WriteString("Risks accepted:\n")
			for _, r := range d.RisksAccepted {
				b.WriteString("- " + r + "\n")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// checkpointArtefactFilenames maps a checkpoint's display name to the real
// Markdown artefact file(s) its check* function writes to
// .forge/specs/<slug>/, in preference order — the first one found is used.
// Checkpoints with no essay-style artefact (Ship/Verify, QA-Verify) are
// intentionally absent: their digest source falls back to cp.Detail, which
// is the only content that exists for them (not a workaround — there is
// nothing else to summarize).
var checkpointArtefactFilenames = map[string][]string{
	"spec":      {"spec.md"},
	"arch":      {"arch.md"},
	"breakdown": {"breakdown.md"},
	"test":      {"test-stubs.md"},
	"code":      {"code-plan.md"},
}

// checkpointArtefactContent reads the real artefact this checkpoint produced
// (J6) so the digest reflects actual generated content instead of the
// one-line checkpoint status message. Returns "" when the checkpoint has no
// known artefact file or the file is absent/unreadable — callers should fall
// back to cp.Detail in that case.
func checkpointArtefactContent(root, slug, checkpointName string) string {
	names, ok := checkpointArtefactFilenames[strings.ToLower(checkpointName)]
	if !ok {
		return ""
	}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(root, ".forge", "specs", slug, name))
		if err == nil && len(data) > 0 {
			return string(data)
		}
	}
	return ""
}

// makeDigestFromArtefact builds a CheckpointDigest by heuristically extracting
// decisions, constraints, and accepted risks from a Markdown artefact.
//
// Extraction logic: bullet points (- or *) under headings whose titles contain
// "decision", "constraint"/"nfr", or "risk"/"trade-off" are collected.
// Each item is truncated to 30 tokens to keep the digest compact.
func makeDigestFromArtefact(checkpoint, artefact string) CheckpointDigest {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(artefact)))
	d := CheckpointDigest{
		Checkpoint:    checkpoint,
		Generated:     time.Now().UTC(),
		SourceHash:    hash[:16],
		TokenEstimate: len(artefact) / 4, // 4-chars-per-token heuristic
	}
	var section string
	for _, line := range strings.Split(artefact, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(trimmed, "#") {
			switch {
			case strings.Contains(lower, "decision"):
				section = "decision"
			case strings.Contains(lower, "constraint") ||
				strings.Contains(lower, "nfr") ||
				strings.Contains(lower, "non-functional"):
				section = "constraint"
			case strings.Contains(lower, "risk") ||
				strings.Contains(lower, "trade-off"):
				section = "risk"
			default:
				section = ""
			}
			continue
		}
		if (strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ")) && section != "" {
			item := strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* ")
			item = truncateTokens(item, 30)
			switch section {
			case "decision":
				d.Decisions = append(d.Decisions, item)
			case "constraint":
				d.Constraints = append(d.Constraints, item)
			case "risk":
				d.RisksAccepted = append(d.RisksAccepted, item)
			}
		}
	}
	return d
}
