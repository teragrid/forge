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

// compound_checkpoint.go — RFC-005 P3: Opt-in compound (named) checkpoints.
//
// A compound checkpoint is a named alias for an ordered list of existing
// checkpoints. For example:
//
//	"quick"     → [spec, arch]
//	"test-only" → [test, qa-verify]
//	"full"      → [spec, arch, test, breakdown, code, ship, qa-verify]
//
// Compounds are declared in forge.yml under ship.compound_checkpoints and
// resolved by ExpandCheckpoints before the pipeline scheduler runs. This
// keeps the scheduler agnostic — it always sees a flat list.
//
// Error code ErrCompoundFailed (FORGE-3218) is raised when a constituent
// checkpoint within a compound fails and the pipeline is not in --yolo mode.
package cmdship

import (
	"fmt"
	"strings"

	"github.com/teragrid/forge/internal/errcode"
)

// ErrCompoundFailed is returned when a constituent checkpoint in a compound
// fails and the pipeline must halt.
var ErrCompoundFailed = errcode.New(
	errcode.Register(errcode.Code(3218), "compound checkpoint failure — constituent checkpoint failed"),
	"compound checkpoint failure — constituent checkpoint failed", nil)

// canonicalCheckpoints is the ordered list of all valid checkpoint names.
var canonicalCheckpoints = []string{
	"spec", "arch", "test", "breakdown", "code", "ship", "qa-verify",
}

// builtinCompounds maps well-known compound names to their constituent lists.
var builtinCompounds = map[string][]string{
	"quick":     {"spec", "arch"},
	"test-only": {"test", "qa-verify"},
	"ci":        {"spec", "arch", "test", "breakdown", "code"},
	"full":      {"spec", "arch", "test", "breakdown", "code", "ship", "qa-verify"},
}

// CompoundRegistry holds user-defined compound checkpoint definitions in
// addition to the builtins. Use NewCompoundRegistry for a pre-loaded instance.
type CompoundRegistry struct {
	defs map[string][]string
}

// NewCompoundRegistry returns a CompoundRegistry pre-populated with built-in
// compounds. User-defined entries (from forge.yml) can be added via Register.
func NewCompoundRegistry() *CompoundRegistry {
	r := &CompoundRegistry{defs: make(map[string][]string)}
	for k, v := range builtinCompounds {
		r.defs[k] = append([]string(nil), v...)
	}
	return r
}

// Register adds (or replaces) a named compound. constituents must all be valid
// canonical checkpoint names or previously registered compound names.
// Returns ErrCompoundFailed when a constituent cannot be resolved.
func (r *CompoundRegistry) Register(name string, constituents []string) error {
	resolved, err := r.expandList(constituents, map[string]bool{name: true})
	if err != nil {
		return fmt.Errorf("%w: register %q: %w", ErrCompoundFailed, name, err)
	}
	r.defs[name] = resolved
	return nil
}

// Expand returns the flat, ordered list of canonical checkpoint names for the
// given checkpoint-or-compound name. Returns (nil, ErrCompoundFailed) for
// unknown names.
func (r *CompoundRegistry) Expand(name string) ([]string, error) {
	return r.expandOne(name, map[string]bool{})
}

// ExpandCheckpoints expands a mixed list of checkpoint and compound names into
// a deduplicated, ordered flat list. Duplicates arising from expansion are
// preserved in order (first occurrence wins).
func (r *CompoundRegistry) ExpandCheckpoints(names []string) ([]string, error) {
	seen := make(map[string]bool)
	var out []string
	for _, n := range names {
		expanded, err := r.Expand(n)
		if err != nil {
			return nil, err
		}
		for _, cp := range expanded {
			if !seen[cp] {
				seen[cp] = true
				out = append(out, cp)
			}
		}
	}
	return out, nil
}

// IsCompound returns true if name is a registered compound (not a canonical
// checkpoint by itself).
func (r *CompoundRegistry) IsCompound(name string) bool {
	_, ok := r.defs[name]
	return ok && !isCanonical(name)
}

// List returns all registered compound names (builtins + user-defined).
func (r *CompoundRegistry) List() []string {
	out := make([]string, 0, len(r.defs))
	for k := range r.defs {
		out = append(out, k)
	}
	return out
}

// ── internal helpers ─────────────────────────────────────────────────────────

func (r *CompoundRegistry) expandOne(name string, visiting map[string]bool) ([]string, error) {
	if isCanonical(name) {
		return []string{name}, nil
	}
	if visiting[name] {
		return nil, fmt.Errorf("cycle detected at compound %q", name)
	}
	constituents, ok := r.defs[name]
	if !ok {
		return nil, fmt.Errorf("unknown checkpoint or compound %q (valid: %s)",
			name, strings.Join(append(canonicalCheckpoints, r.List()...), ", "))
	}
	visiting[name] = true
	defer delete(visiting, name)
	return r.expandList(constituents, visiting)
}

func (r *CompoundRegistry) expandList(names []string, visiting map[string]bool) ([]string, error) {
	var out []string
	for _, n := range names {
		expanded, err := r.expandOne(n, visiting)
		if err != nil {
			return nil, err
		}
		out = append(out, expanded...)
	}
	return out, nil
}

func isCanonical(name string) bool {
	for _, c := range canonicalCheckpoints {
		if c == name {
			return true
		}
	}
	return false
}

// DefaultCompoundRegistry is the package-level registry used by the pipeline.
var DefaultCompoundRegistry = NewCompoundRegistry()
