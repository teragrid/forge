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

// incremental_rerun.go — RFC-005 P3: Diff-aware incremental re-run.
//
// IncrementalPlan computes the minimal set of checkpoints that must re-run
// given changes since the last successful pipeline execution. Each checkpoint
// has a set of "watched" input files; if any watched file is newer than the
// checkpoint's snapshot the checkpoint is included in the re-run plan.
//
// The baseline is the most recent snapshot directory under
// .forge/.snapshots/<slug>/. If no baseline exists, all checkpoints are
// returned (full run).
//
// Dependency rules (topological):
//
//	spec      — watches: .forge/specs/<slug>/spec.md (always a source)
//	arch      — depends on: spec; watches: spec.md + arch.md
//	test      — depends on: spec; watches: spec.md + test-spec.md
//	breakdown — depends on: arch; watches: arch.md + tasks.md
//	code      — depends on: breakdown; watches: tasks.md + code-plan.md
//	ship      — depends on: code; watches: branch state (always re-runs if code did)
//	qa-verify — depends on: ship; watches: traceability.yaml + qa-report.md
package cmdship

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/teragrid/forge/internal/errcode"
)

// ErrIncrementalNoBaseline is returned when IncrementalPlan is called but no
// prior snapshot exists and the caller requested strict mode.
var ErrIncrementalNoBaseline = errcode.New(
	errcode.Register(errcode.Code(3219), "incremental re-run: no baseline snapshot found — run full pipeline first"),
	"incremental re-run: no baseline snapshot found — run full pipeline first", nil)

// checkpointWatches maps each checkpoint to the spec-dir-relative files it
// reads as primary inputs. A change to any watched file invalidates the
// checkpoint and all its dependents.
var checkpointWatches = map[string][]string{
	"spec":      {"spec.md", "spec.yml"},
	"arch":      {"spec.md", "arch.md"},
	"test":      {"spec.md", "test-spec.md", "traceability.yaml"},
	"breakdown": {"arch.md", "tasks.md"},
	"code":      {"tasks.md", "code-plan.md"},
	"ship":      {"code-plan.md"},
	"qa-verify": {"traceability.yaml", "qa-report.md"},
}

// checkpointDeps maps each checkpoint to the checkpoints it depends on.
// A checkpoint is re-run whenever any of its own watches changed OR any
// dependency is in the re-run set.
var checkpointDeps = map[string][]string{
	"spec":      {},
	"arch":      {"spec"},
	"test":      {"spec"},
	"breakdown": {"arch"},
	"code":      {"breakdown"},
	"ship":      {"code"},
	"qa-verify": {"ship"},
}

// IncrementalPlanResult is returned by IncrementalPlan.
type IncrementalPlanResult struct {
	// Slug is the feature whose checkpoints were evaluated.
	Slug string
	// Rerun is the ordered list of checkpoints that must re-run.
	Rerun []string
	// Skip is the ordered list of checkpoints that can be safely skipped.
	Skip []string
	// BaselineFound is true when a prior snapshot existed to diff against.
	BaselineFound bool
	// Reason maps each checkpoint name to the human-readable reason for
	// including or skipping it.
	Reason map[string]string
}

// IncrementalPlan returns the minimal checkpoint list needed given changes
// since the last snapshot. Pass strictNoBaseline=true to return
// ErrIncrementalNoBaseline when no snapshot exists (useful in CI).
func IncrementalPlan(root, slug string, strictNoBaseline bool) (*IncrementalPlanResult, error) {
	specDir := filepath.Join(root, ".forge", "specs", slug)
	snapBase := filepath.Join(root, ".forge", snapshotsBaseDir, slug)

	result := &IncrementalPlanResult{
		Slug:   slug,
		Reason: make(map[string]string),
	}

	// Determine whether a baseline snapshot exists.
	snapshotTimes, err := readSnapshotTimes(snapBase)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("incremental: read snapshots: %w", err)
	}
	result.BaselineFound = len(snapshotTimes) > 0

	if !result.BaselineFound {
		if strictNoBaseline {
			return nil, ErrIncrementalNoBaseline
		}
		// No baseline → run everything.
		result.Rerun = append([]string(nil), canonicalCheckpoints...)
		result.Skip = nil
		for _, cp := range canonicalCheckpoints {
			result.Reason[cp] = "no baseline snapshot — full run required"
		}
		return result, nil
	}

	// Determine which checkpoints are dirty (watched file newer than snapshot).
	dirty := make(map[string]bool)
	for _, cp := range canonicalCheckpoints {
		snapshotAt, hasSnap := snapshotTimes[cp]
		if !hasSnap {
			dirty[cp] = true
			result.Reason[cp] = "checkpoint never run before"
			continue
		}
		watches := checkpointWatches[cp]
		for _, rel := range watches {
			abs := filepath.Join(specDir, rel)
			info, err := os.Stat(abs)
			if err != nil {
				continue
			}
			if info.ModTime().After(snapshotAt) {
				dirty[cp] = true
				result.Reason[cp] = fmt.Sprintf("watched file %s modified after last snapshot (%s)",
					rel, snapshotAt.Format("15:04:05"))
				break
			}
		}
	}

	// Propagate dirtiness through dependencies (topological order).
	for _, cp := range canonicalCheckpoints {
		if dirty[cp] {
			continue
		}
		for _, dep := range checkpointDeps[cp] {
			if dirty[dep] {
				dirty[cp] = true
				result.Reason[cp] = fmt.Sprintf("dependency %q is dirty", dep)
				break
			}
		}
	}

	// Build ordered lists.
	for _, cp := range canonicalCheckpoints {
		if dirty[cp] {
			result.Rerun = append(result.Rerun, cp)
			if result.Reason[cp] == "" {
				result.Reason[cp] = "dirty"
			}
		} else {
			result.Skip = append(result.Skip, cp)
			result.Reason[cp] = "no changes detected — skipped"
		}
	}
	return result, nil
}

// readSnapshotTimes returns a map of checkpoint → snapshot creation time by
// reading the meta.txt (or directory mtime as fallback) under snapBase.
func readSnapshotTimes(snapBase string) (map[string]time.Time, error) {
	entries, err := os.ReadDir(snapBase)
	if err != nil {
		return nil, err
	}
	out := make(map[string]time.Time, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out[e.Name()] = info.ModTime()
	}
	return out, nil
}

// CheckpointNeedsRerun is a convenience wrapper: returns true when the given
// checkpoint appears in IncrementalPlan's Rerun list.
func CheckpointNeedsRerun(root, slug, checkpoint string) (bool, error) {
	plan, err := IncrementalPlan(root, slug, false)
	if err != nil {
		return true, err // fail open: re-run when uncertain
	}
	for _, cp := range plan.Rerun {
		if cp == checkpoint {
			return true, nil
		}
	}
	return false, nil
}
