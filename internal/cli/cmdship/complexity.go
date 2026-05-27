package cmdship

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// ComplexityTier names the four operational buckets that determine which
// checkpoints run and how large the LLM token budgets are.
type ComplexityTier string

const (
	// ComplexityNano skips most checkpoints (~5,000 tokens total).
	ComplexityNano ComplexityTier = "nano"
	// ComplexityMicro runs 4 checkpoints without debate rounds (~22,000 tokens).
	ComplexityMicro ComplexityTier = "micro"
	// ComplexityStandard runs all 7 checkpoints with light debate (~50,000 tokens).
	ComplexityStandard ComplexityTier = "standard"
	// ComplexityComplex runs 7 checkpoints + domain gates + full debate (~90,000 tokens).
	ComplexityComplex ComplexityTier = "complex"
)

// complexityScore returns a raw score from the signals; 0 = simplest.
//
//	Score thresholds  → Tier
//	≥ 60              → complex
//	≥ 30              → standard
//	≥ 10              → micro
//	< 10              → nano
const (
	scoreMigration  = 40
	scoreCompliance = 40
	scoreNewService = 25
	scoreExternal   = 20
	scoreWords100   = 15
)

// ComplexitySignal is a decomposed view of every classifier input; exposed so
// tests can assert individual fields without parsing the description again.
type ComplexitySignal struct {
	DescriptionWords   int
	MentionsNewService bool
	MentionsMigration  bool
	MentionsCompliance bool
	MentionsExternal   bool
	HasSpecYML         bool
	ExistingArtefacts  int // number of .forge/specs/<slug>/*.md files already present
}

// classifyComplexity scores the feature description and workspace artefacts,
// then maps the result to a ComplexityTier.  root is the repository root.
func classifyComplexity(desc string, root string) ComplexityTier {
	sig := collectSignals(desc, root)
	score := computeScore(sig)
	return tierFromScore(score)
}

// ClassifyComplexityWithSignals is the testable variant that also returns the
// raw signal and the numeric score.
func ClassifyComplexityWithSignals(desc string, root string) (ComplexityTier, ComplexitySignal, int) {
	sig := collectSignals(desc, root)
	score := computeScore(sig)
	return tierFromScore(score), sig, score
}

// collectSignals builds a ComplexitySignal from the description and the
// workspace on disk.  All disk reads are best-effort; failures are silent.
func collectSignals(desc string, root string) ComplexitySignal {
	lower := strings.ToLower(desc)

	sig := ComplexitySignal{
		DescriptionWords:   countWords(desc),
		MentionsNewService: containsAny(lower, "new service", "microservice", "new api", "new endpoint"),
		MentionsMigration:  containsAny(lower, "migrat", "upgrade", "refactor", "rewrite"),
		MentionsCompliance: containsAny(lower, "compliance", "gdpr", "hipaa", "pci", "soc2", "audit"),
		MentionsExternal:   containsAny(lower, "third-party", "third party", "external api", "webhook", "integration"),
	}

	slug := slugify(desc)
	specsDir := filepath.Join(root, ".forge", "specs", slug)

	// Check for spec.yml (indicates previous pipeline run artifacts exist).
	if _, err := os.Stat(filepath.Join(specsDir, "spec.yml")); err == nil {
		sig.HasSpecYML = true
	}

	// Count existing artefact .md files in the specs dir.
	if entries, err := os.ReadDir(specsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				sig.ExistingArtefacts++
			}
		}
	}

	return sig
}

// computeScore sums the weighted signals into a single integer.
func computeScore(sig ComplexitySignal) int {
	score := 0
	if sig.MentionsMigration {
		score += scoreMigration
	}
	if sig.MentionsCompliance {
		score += scoreCompliance
	}
	if sig.MentionsNewService {
		score += scoreNewService
	}
	if sig.MentionsExternal {
		score += scoreExternal
	}
	if sig.DescriptionWords > 100 {
		score += scoreWords100
	}
	return score
}

// tierFromScore converts a numeric score to a ComplexityTier.
func tierFromScore(score int) ComplexityTier {
	switch {
	case score >= 60:
		return ComplexityComplex
	case score >= 30:
		return ComplexityStandard
	case score >= 10:
		return ComplexityMicro
	default:
		return ComplexityNano
	}
}

// countWords returns the number of whitespace-delimited words in s.
func countWords(s string) int {
	inWord := false
	count := 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
	}
	return count
}

// containsAny reports whether s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
