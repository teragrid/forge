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

package knowledge

import "sort"

// Scoring weights — tuned to keep ≤5 highly relevant entries per call.
const (
	weightShipCheckpoint = 3 // exact ship_checkpoints match
	weightScanFamily     = 3 // exact scan_families match
	weightTemplate       = 2 // exact templates match
	weightTag            = 1 // per-tag overlap

	// DefaultMaxN is the maximum number of entries returned by Select.
	DefaultMaxN = 5

	// MinScore is the minimum score for an entry to be included.
	MinScore = 1
)

// Score computes a relevance score for e given the query parameters.
// Higher is more relevant. Returns 0 when nothing matches.
func Score(e Entry, checkpoint, family, tmpl string, queryTags []string) int {
	score := 0

	for _, cp := range e.ForgeIntegration.ShipCheckpoints {
		if cp == checkpoint {
			score += weightShipCheckpoint
			break
		}
	}
	for _, sf := range e.ForgeIntegration.ScanFamilies {
		if sf == family {
			score += weightScanFamily
			break
		}
	}
	for _, t := range e.ForgeIntegration.Templates {
		if t == tmpl {
			score += weightTemplate
			break
		}
	}

	tagSet := make(map[string]bool, len(e.Tags))
	for _, t := range e.Tags {
		tagSet[t] = true
	}
	for _, qt := range queryTags {
		if tagSet[qt] {
			score += weightTag
		}
	}

	return score
}

// Select returns the top-N (≤DefaultMaxN) entries from idx with Score ≥ MinScore,
// sorted by descending score. A nil idx returns nil.
func Select(idx *Index, checkpoint, family, tmpl string, tags []string) []Entry {
	if idx == nil {
		return nil
	}

	type scored struct {
		e Entry
		s int
	}

	candidates := make([]scored, 0, 16)
	for _, e := range idx.Entries {
		if s := Score(e, checkpoint, family, tmpl, tags); s >= MinScore {
			candidates = append(candidates, scored{e, s})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].s > candidates[j].s
	})

	n := DefaultMaxN
	if len(candidates) < n {
		n = len(candidates)
	}

	out := make([]Entry, n)
	for i := range n {
		out[i] = candidates[i].e
	}
	return out
}

// SelectForShip is a convenience wrapper for forge ship checkpoints.
// tags is derived from the feature description tokens.
func SelectForShip(idx *Index, checkpoint string, tags []string) []Entry {
	return Select(idx, checkpoint, "", "", tags)
}

// SelectForScan is a convenience wrapper for forge scan families.
func SelectForScan(idx *Index, family string) []Entry {
	return Select(idx, "", family, "", nil)
}

// SelectForNew is a convenience wrapper for forge new templates.
func SelectForNew(idx *Index, templateName string) []Entry {
	return Select(idx, "", "", templateName, nil)
}
