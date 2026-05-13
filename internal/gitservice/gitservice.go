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
	"os/exec"
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

// run executes a read-only git subcommand in s.root.
func (s *Service) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...) //nolint:gosec // args are caller-controlled
	cmd.Dir = s.root
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
