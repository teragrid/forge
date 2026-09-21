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

// Package gitservice implements DEV-M0-06: a read-only Git service that wraps
// the `git` binary. All operations are strictly read-only; the package provides
// no write paths (verified by the absence of any git write subcommands).
//
// All commands are run inside the repository root via exec.Command and the
// output is parsed into structured types. The service returns FORGE-2600 if
// the working directory is not inside a Git repository.
package gitservice

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/teragrid/forge/internal/errcode"
)

// Reserved error codes (range 2600..2699).
var (
	ErrNotGitRepo  = errcode.Register(errcode.Code(2600), "directory is not a git repository")
	ErrGitNotFound = errcode.Register(errcode.Code(2601), "git binary not found in PATH")
	ErrGitFailed   = errcode.Register(errcode.Code(2602), "git command failed")
)

// FileStatus represents one entry in `git status --porcelain`.
type FileStatus struct {
	XY   string // two-char porcelain status code
	Path string // repo-relative path
}

// Commit represents a single commit from `git log`.
type Commit struct {
	Hash    string
	Author  string
	Date    time.Time
	Subject string
}

// DiffStat summarises `git diff --stat` output.
type DiffStat struct {
	Files     int
	Additions int
	Deletions int
	Lines     []string // raw diff --stat lines
}

// Service is a read-only Git service bound to a repository root.
type Service struct {
	root string
}

// New returns a Service rooted at dir. Returns ErrNotGitRepo if dir is not
// inside a git repo, or ErrGitNotFound if the git binary is unavailable.
func New(dir string) (*Service, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, errcode.New(ErrGitNotFound, "git not found in PATH", err)
	}
	s := &Service{root: dir}
	// Verify we are inside a repo.
	if _, err := s.run("rev-parse", "--git-dir"); err != nil {
		return nil, errcode.New(ErrNotGitRepo, dir+" is not a git repository", err)
	}
	return s, nil
}

// Status returns the working-tree status (equivalent to `git status --porcelain`).
func (s *Service) Status() ([]FileStatus, error) {
	out, err := s.run("status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var result []FileStatus
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 4 {
			continue
		}
		result = append(result, FileStatus{
			XY:   line[:2],
			Path: strings.TrimSpace(line[3:]),
		})
	}
	return result, nil
}

// Log returns up to n commits from HEAD. Pass n=0 for the default (20).
func (s *Service) Log(n int) ([]Commit, error) {
	if n <= 0 {
		n = 20
	}
	format := "--pretty=format:%H%x1f%an%x1f%aI%x1f%s"
	out, err := s.run("log", fmt.Sprintf("-n%d", n), format)
	if err != nil {
		return nil, err
	}
	var commits []Commit
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\x1f")
		if len(parts) != 4 {
			continue
		}
		t, _ := time.Parse(time.RFC3339, parts[2])
		commits = append(commits, Commit{
			Hash:    parts[0],
			Author:  parts[1],
			Date:    t,
			Subject: parts[3],
		})
	}
	return commits, nil
}

// DiffSince returns the diff stat between ref and HEAD.
// ref may be a commit hash, branch name, or tag.
func (s *Service) DiffSince(ref string) (*DiffStat, error) {
	if ref == "" {
		return nil, errors.New("gitservice: ref must not be empty")
	}
	out, err := s.run("diff", "--stat", ref, "HEAD")
	if err != nil {
		return nil, err
	}
	stat := &DiffStat{}
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		stat.Lines = append(stat.Lines, line)
	}
	// Parse the summary line (last line): "3 files changed, 10 insertions(+), 2 deletions(-)"
	if len(stat.Lines) > 0 {
		summary := stat.Lines[len(stat.Lines)-1]
		fmt.Sscanf(summary, " %d file", &stat.Files)                                       //nolint:errcheck
		fmt.Sscanf(summary, "%*d file%*s, %d insertion", &stat.Additions)                  //nolint:errcheck
		fmt.Sscanf(summary, "%*d file%*s, %*d insertion%*s, %d deletion", &stat.Deletions) //nolint:errcheck
	}
	return stat, nil
}

// ChangedFilesSince returns the list of files changed between ref and HEAD.
func (s *Service) ChangedFilesSince(ref string) ([]string, error) {
	if ref == "" {
		return nil, errors.New("gitservice: ref must not be empty")
	}
	out, err := s.run("diff", "--name-only", ref, "HEAD")
	if err != nil {
		return nil, err
	}
	var files []string
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		if f := strings.TrimSpace(scanner.Text()); f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

// baseRefCandidates are tried in order by ChangedFilesOnBranch. Remote-tracking
// refs come first: a local main that lags origin would understate the branch's
// own work, and one that is ahead of origin would overstate it.
var baseRefCandidates = []string{"origin/main", "origin/master", "main", "master"}

// ChangedFilesOnBranch returns the files changed on the current branch relative
// to the repository's default branch (merge-base diff, so commits already on
// the default branch are not counted), plus any uncommitted paths.
//
// ok is false when no base ref can be resolved — a repo with no main/master, a
// detached HEAD on the base itself, or git failing. Callers must treat that as
// "unknown", never as "nothing changed": reporting a fact forge could not
// establish is the false-green this exists to prevent.
func (s *Service) ChangedFilesOnBranch() (files []string, ok bool) {
	seen := make(map[string]bool)
	add := func(f string) {
		f = strings.TrimSpace(f)
		if f != "" && !seen[f] {
			seen[f] = true
			files = append(files, f)
		}
	}
	for _, base := range baseRefCandidates {
		if _, err := s.run("rev-parse", "--verify", "--quiet", base+"^{commit}"); err != nil {
			continue
		}
		out, err := s.run("diff", "--name-only", base+"...HEAD")
		if err != nil {
			continue
		}
		for _, line := range strings.Split(out, "\n") {
			add(line)
		}
		ok = true
		break
	}
	if !ok {
		return nil, false
	}
	if statuses, err := s.Status(); err == nil {
		for _, st := range statuses {
			add(st.Path)
		}
	}
	return files, true
}

// GoFileCommitTimes returns the last-commit timestamp for each Go source file
// that has been committed to this repository. Map keys are repo-relative paths
// with forward slashes. Files with no commits (new/untracked) are absent.
// A single batch git log call is used for efficiency.
func (s *Service) GoFileCommitTimes() map[string]time.Time {
	out, err := s.run("log", "--format=%ct", "--name-only", "--diff-filter=ACMR", "--", "*.go")
	if err != nil {
		return nil
	}
	result := make(map[string]time.Time)
	var curTS int64
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		// Lines that are pure integers are commit-timestamp headers.
		if t, parseErr := strconv.ParseInt(line, 10, 64); parseErr == nil {
			curTS = t
			continue
		}
		// Otherwise the line is a file path; record only the first (most-recent) occurrence.
		p := filepath.ToSlash(strings.TrimSpace(line))
		if _, exists := result[p]; !exists && curTS > 0 {
			result[p] = time.Unix(curTS, 0)
		}
	}
	return result
}

// run executes a read-only git subcommand in s.root.
func (s *Service) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...) //nolint:gosec // args are caller-controlled
	cmd.Dir = s.root
	cmd.Env = scrubbedGitEnv(os.Environ())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", errcode.New(ErrGitFailed,
			fmt.Sprintf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String())),
			err)
	}
	return stdout.String(), nil
}

// repoPointingEnv are the variables git exports to hooks (and honours from any
// parent) that redirect it to a specific repository, index or object store.
var repoPointingEnv = []string{
	"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_PREFIX", "GIT_COMMON_DIR",
	"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_NAMESPACE",
}

// scrubbedGitEnv returns env without the variables that would make git ignore
// the directory the Service was opened on. A Service is created for an explicit
// root; when forge runs inside a git hook (or under `git rebase --exec`) the
// inherited GIT_DIR would otherwise silently point every command at the hook's
// repository instead — reporting that repository's status and history for the
// wrong directory.
func scrubbedGitEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		drop := false
		for _, name := range repoPointingEnv {
			if strings.HasPrefix(kv, name+"=") {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, kv)
		}
	}
	return out
}
