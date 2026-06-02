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

// drift_detect.go — RFC-005 P3: Continuous spec-vs-code drift detection.
//
// DetectDrift compares the spec artefacts under .forge/specs/<slug>/ with the
// implementation files and reports divergence signals:
//
//  1. ModTime drift — spec.md modified more recently than any code file in the
//     slug's task list (code hasn't caught up with the spec).
//  2. AC count drift — number of ACs in spec.md differs from the last recorded
//     count in .forge/specs/<slug>/drift-baseline.json.
//  3. Task completeness drift — tasks.md has unchecked items AND no git commit
//     was made more recently than the last snapshot (stale incomplete tasks).
//
// The report is also written to .forge/specs/<slug>/drift-report.json for
// consumption by CI and `forge ship --resume`.
package cmdship

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/teragrid/forge/internal/errcode"
)

// ErrDriftDetected is raised when DetectDrift finds divergence above the
// configured threshold.
var ErrDriftDetected = errcode.New(
	errcode.Register(errcode.Code(3217), "drift detected — spec-vs-code divergence exceeds threshold"),
	"drift detected — spec-vs-code divergence exceeds threshold", nil)

// DriftSignal is a single detected divergence signal.
type DriftSignal struct {
	// Kind is the signal type: "modtime" | "ac-count" | "task-completeness".
	Kind string `json:"kind"`
	// Severity is "warning" or "blocking".
	Severity string `json:"severity"`
	// Detail is a human-readable description of the divergence.
	Detail string `json:"detail"`
}

// DriftReport is the full output of DetectDrift.
type DriftReport struct {
	Slug       string        `json:"slug"`
	CheckedAt  time.Time     `json:"checked_at"`
	Signals    []DriftSignal `json:"signals"`
	BlockCount int           `json:"block_count"`
	WarnCount  int           `json:"warn_count"`
	// Clean is true when no signals were found.
	Clean bool `json:"clean"`
}

// DriftBaseline records the last known-good counts so future checks can detect
// divergence. Written to .forge/specs/<slug>/drift-baseline.json.
type DriftBaseline struct {
	Slug       string    `json:"slug"`
	RecordedAt time.Time `json:"recorded_at"`
	ACCount    int       `json:"ac_count"`
	TaskTotal  int       `json:"task_total"`
	TaskDone   int       `json:"task_done"`
}

// DetectDrift scans the spec and returns a DriftReport. The report is written
// to .forge/specs/<slug>/drift-report.json as a side effect. Returns
// ErrDriftDetected when BlockCount > 0 and failOnBlock is true.
func DetectDrift(root, slug string, failOnBlock bool) (*DriftReport, error) {
	specDir := filepath.Join(root, ".forge", "specs", slug)
	report := &DriftReport{
		Slug:      slug,
		CheckedAt: time.Now().UTC(),
	}

	// 1. Gather spec.md modification time.
	specMD := filepath.Join(specDir, "spec.md")
	specStat, err := os.Stat(specMD)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			report.Clean = true
			return report, nil // nothing to compare yet
		}
		return nil, fmt.Errorf("drift detect: stat spec.md: %w", err)
	}

	// 2. Count ACs in spec.md.
	acCount, err := countACsInSpec(specMD)
	if err != nil {
		return nil, fmt.Errorf("drift detect: count ACs: %w", err)
	}

	// 3. Read baseline (if present) and compare AC count.
	baseline, _ := readDriftBaseline(specDir)
	if baseline != nil && baseline.ACCount > 0 && acCount != baseline.ACCount {
		delta := acCount - baseline.ACCount
		sign := "+"
		if delta < 0 {
			sign = ""
		}
		report.Signals = append(report.Signals, DriftSignal{
			Kind:     "ac-count",
			Severity: "warning",
			Detail: fmt.Sprintf("AC count changed %s%d (was %d, now %d) since baseline %s",
				sign, delta, baseline.ACCount, acCount, baseline.RecordedAt.Format("2006-01-02")),
		})
	}

	// 4. Parse tasks.md for completeness.
	taskTotal, taskDone, err := countTasksInBreakdown(filepath.Join(specDir, "tasks.md"))
	if err == nil && taskTotal > 0 {
		unchecked := taskTotal - taskDone
		if unchecked > 0 {
			// Check whether spec.md was modified after any recent snapshot.
			// If spec is newer than last snapshot → tasks should have been updated.
			snapDir := filepath.Join(root, ".forge", snapshotsBaseDir, slug)
			latestSnap := latestSnapshotTime(snapDir)
			if specStat.ModTime().After(latestSnap) {
				report.Signals = append(report.Signals, DriftSignal{
					Kind:     "task-completeness",
					Severity: "blocking",
					Detail: fmt.Sprintf("%d of %d tasks unchecked in tasks.md — spec updated after last snapshot",
						unchecked, taskTotal),
				})
			} else {
				report.Signals = append(report.Signals, DriftSignal{
					Kind:     "task-completeness",
					Severity: "warning",
					Detail:   fmt.Sprintf("%d of %d tasks still unchecked in tasks.md", unchecked, taskTotal),
				})
			}
		}
	}

	// 5. Modtime drift: spec updated more recently than code files.
	codeNewerThanSpec, codeFiles := anyCodeFileNewerThan(root, slug, specStat.ModTime())
	if !codeNewerThanSpec && len(codeFiles) == 0 {
		// No code files found at all — spec is ahead.
		if taskTotal > 0 && taskDone < taskTotal {
			report.Signals = append(report.Signals, DriftSignal{
				Kind:     "modtime",
				Severity: "warning",
				Detail:   "spec.md has been updated but no implementation files found for this slug",
			})
		}
	}

	// Tally severities.
	for _, s := range report.Signals {
		if s.Severity == "blocking" {
			report.BlockCount++
		} else {
			report.WarnCount++
		}
	}
	report.Clean = len(report.Signals) == 0

	// Write report to disk (best-effort; ignore error — non-critical).
	_ = writeJSON(filepath.Join(specDir, "drift-report.json"), report)

	if failOnBlock && report.BlockCount > 0 {
		return report, ErrDriftDetected
	}
	return report, nil
}

// SaveDriftBaseline records the current AC and task counts as the new baseline.
func SaveDriftBaseline(root, slug string) error {
	specDir := filepath.Join(root, ".forge", "specs", slug)
	specMD := filepath.Join(specDir, "spec.md")
	acCount, err := countACsInSpec(specMD)
	if err != nil {
		return fmt.Errorf("drift baseline: count ACs: %w", err)
	}
	taskTotal, taskDone, _ := countTasksInBreakdown(filepath.Join(specDir, "tasks.md"))
	bl := DriftBaseline{
		Slug:       slug,
		RecordedAt: time.Now().UTC(),
		ACCount:    acCount,
		TaskTotal:  taskTotal,
		TaskDone:   taskDone,
	}
	return writeJSON(filepath.Join(specDir, "drift-baseline.json"), bl)
}

// ── helpers ──────────────────────────────────────────────────────────────────

// countACsInSpec counts lines matching "- [ ]" or "- [x]" AC patterns.
func countACsInSpec(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	count := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "- [ ]") || strings.HasPrefix(line, "- [x]") ||
			strings.HasPrefix(line, "- [X]") {
			count++
		}
	}
	return count, sc.Err()
}

// countTasksInBreakdown counts total and done task entries in tasks.md.
func countTasksInBreakdown(path string) (total, done int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "- [ ]") {
			total++
		} else if strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [X]") {
			total++
			done++
		}
	}
	return total, done, sc.Err()
}

// latestSnapshotTime returns the ModTime of the newest entry in snapDir,
// or the zero time when the directory is absent or empty.
func latestSnapshotTime(snapDir string) time.Time {
	entries, err := os.ReadDir(snapDir)
	if err != nil {
		return time.Time{}
	}
	var latest time.Time
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	return latest
}

// anyCodeFileNewerThan checks whether any *.go / *.ts / *.py file under
// root/internal or root/src is newer than t. Returns true + list of newer files.
func anyCodeFileNewerThan(root, slug string, t time.Time) (bool, []string) {
	var newer []string
	codeDirs := []string{
		filepath.Join(root, "internal"),
		filepath.Join(root, "src"),
		filepath.Join(root, "lib"),
	}
	extensions := map[string]bool{
		".go": true, ".ts": true, ".py": true, ".java": true, ".rs": true,
	}
	for _, dir := range codeDirs {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, werr error) error {
			if werr != nil || d.IsDir() {
				return nil
			}
			if !extensions[filepath.Ext(path)] {
				return nil
			}
			// Only care about files whose path contains the slug.
			if !strings.Contains(strings.ToLower(filepath.Base(path)), strings.ToLower(slug)) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if info.ModTime().After(t) {
				newer = append(newer, path)
			}
			return nil
		})
	}
	return len(newer) > 0, newer
}

// readDriftBaseline reads the baseline from disk, returns nil if absent.
func readDriftBaseline(specDir string) (*DriftBaseline, error) {
	data, err := os.ReadFile(filepath.Join(specDir, "drift-baseline.json"))
	if err != nil {
		return nil, err
	}
	var bl DriftBaseline
	if err := json.Unmarshal(data, &bl); err != nil {
		return nil, err
	}
	return &bl, nil
}

// writeJSON marshals v to path. Creates parent directories as needed.
func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
