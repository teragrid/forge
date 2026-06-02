// test_scoring.go — 9-dimension test quality scoring rubric.
// RFC-005 §6.2.
package cmdship

import "fmt"

// Dimension identifies one of the 9 test quality dimensions.
type Dimension int

const (
	D1HappyPath     Dimension = iota + 1 // weight 1.0×, threshold ≥7
	D2Boundary                           // weight 1.5×, threshold ≥6
	D3Negative                           // weight 1.5×, threshold ≥6
	D4Idempotency                        // weight 1.2×, threshold ≥5
	D5Concurrency                        // weight 1.2×, threshold ≥5
	D6AuthZ                              // weight 2.0×, threshold ≥8 (T2: ≥9)
	D7Regression                         // weight 1.0×, threshold ≥7 (bug-fix: ≥8)
	D8DataAccuracy                       // weight 1.3×, threshold ≥6
	D9FalsePositive                      // weight 1.2×, threshold ≥6
)

// dimensionMeta holds the scoring metadata for each dimension.
var dimensionMeta = map[Dimension]struct {
	Weight    float64
	Threshold float64
}{
	D1HappyPath:     {1.0, 7.0},
	D2Boundary:      {1.5, 6.0},
	D3Negative:      {1.5, 6.0},
	D4Idempotency:   {1.2, 5.0},
	D5Concurrency:   {1.2, 5.0},
	D6AuthZ:         {2.0, 8.0},
	D7Regression:    {1.0, 7.0},
	D8DataAccuracy:  {1.3, 6.0},
	D9FalsePositive: {1.2, 6.0},
}

// DimensionScore is the score awarded to a single dimension.
type DimensionScore struct {
	Dim     Dimension
	Score   float64 // 0–10 scale
	Covered bool    // true when at least one test targets this dimension
}

// ScoreResult is the output of ComputeCompositeScore.
type ScoreResult struct {
	Scores          []DimensionScore
	CompositeScore  float64
	GateStatus      string      // "PASS" | "WARNING" | "BLOCK"
	MissingDims     []Dimension // dimensions with Covered=false (not waived)
	WaivedDims      []Dimension
	BlockingReasons []string
}

// ComputeCompositeScore computes the weighted composite score and gate status.
//
// Parameters:
//   - scores:   one DimensionScore per dimension (must be in D1..D9 order)
//   - tier:     "T0" | "T1" | "T2" (affects D6 threshold and waiver rules)
//   - isBugFix: true raises D7 threshold from 7 to 8
//   - waivers:  dimensions explicitly waived with justification (T0 allows D5/D6/D8)
//
// RFC-005 §6.2.
func ComputeCompositeScore(scores []DimensionScore, tier string, isBugFix bool, waivers []Dimension) ScoreResult {
	waiverSet := make(map[Dimension]bool, len(waivers))
	for _, d := range waivers {
		waiverSet[d] = true
	}

	// Build effective thresholds with tier/bug-fix overrides.
	effectiveMeta := make(map[Dimension]struct{ Weight, Threshold float64 }, 9)
	for d, m := range dimensionMeta {
		em := struct{ Weight, Threshold float64 }{m.Weight, m.Threshold}
		if d == D6AuthZ && tier == "T2" {
			em.Threshold = 9.0
		}
		if d == D7Regression && isBugFix {
			em.Threshold = 8.0
		}
		effectiveMeta[d] = em
	}

	var weightedSum, totalWeight float64
	var missing, waived []Dimension
	var blocking []string

	for _, s := range scores {
		meta, ok := effectiveMeta[s.Dim]
		if !ok {
			continue
		}

		if waiverSet[s.Dim] {
			waived = append(waived, s.Dim)
			// Waived dimensions do not contribute to the composite score.
			continue
		}

		weightedSum += s.Score * meta.Weight
		totalWeight += meta.Weight

		if !s.Covered {
			missing = append(missing, s.Dim)
		}

		// Check threshold violation for high-weight dimensions (weight ≥ 1.5).
		if meta.Weight >= 1.5 && s.Score < meta.Threshold {
			blocking = append(blocking, dimensionBlockReason(s.Dim, s.Score, meta.Threshold, tier, isBugFix))
		}

		// In bug-fix mode D7 Regression Guard threshold is raised to 8 and is
		// always enforced as a BLOCK condition, regardless of its base weight (1.0).
		if isBugFix && s.Dim == D7Regression && s.Score < meta.Threshold {
			blocking = append(blocking, dimensionBlockReason(s.Dim, s.Score, meta.Threshold, tier, isBugFix))
		}
	}

	composite := 0.0
	if totalWeight > 0 {
		composite = weightedSum / totalWeight
	}

	// Gate determination:
	//   BLOCK  when composite < 6.5 OR any high-weight dim below threshold
	//   WARNING when any dim uncovered (but not BLOCK)
	//   PASS   otherwise
	status := "PASS"
	if composite < 6.5 {
		blocking = append(blocking, "composite score below 6.5 threshold")
	}
	if len(blocking) > 0 {
		status = "BLOCK"
	} else if len(missing) > 0 {
		status = "WARNING"
	}

	return ScoreResult{
		Scores:          scores,
		CompositeScore:  composite,
		GateStatus:      status,
		MissingDims:     missing,
		WaivedDims:      waived,
		BlockingReasons: blocking,
	}
}

// dimensionBlockReason returns a human-readable blocking reason string.
func dimensionBlockReason(d Dimension, score, threshold float64, tier string, isBugFix bool) string {
	name := dimensionName(d)
	note := ""
	if d == D6AuthZ && tier == "T2" {
		note = " (T2 threshold)"
	}
	if d == D7Regression && isBugFix {
		note = " (bug-fix mode)"
	}
	return fmt.Sprintf("%s score %.1f below required %.1f%s", name, score, threshold, note)
}

// dimensionName returns the human-readable label for a dimension.
func dimensionName(d Dimension) string {
	switch d {
	case D1HappyPath:
		return "D1-HappyPath"
	case D2Boundary:
		return "D2-Boundary"
	case D3Negative:
		return "D3-Negative"
	case D4Idempotency:
		return "D4-Idempotency"
	case D5Concurrency:
		return "D5-Concurrency"
	case D6AuthZ:
		return "D6-AuthZ"
	case D7Regression:
		return "D7-Regression"
	case D8DataAccuracy:
		return "D8-DataAccuracy"
	case D9FalsePositive:
		return "D9-FalsePositive"
	default:
		return "D?-Unknown"
	}
}
