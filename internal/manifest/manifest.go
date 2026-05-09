// Package manifest reads the `.forge/manifest` file used by `forge clean` to
// distinguish managed vs unmanaged paths in a project (DEV-M0-15 / Spec §4
// hygiene).
//
// MVP format (intentionally minimal; M1 promotes to YAML per spec):
//
//	# Lines starting with '#' are comments. Blanks ignored.
//	[scratch]
//	**/.forge/scratch/**
//	**/*.tmp.*
//	**/_scratch_*
//
//	[managed]
//	.gitignore
//	.gitleaks.toml
//	.forge/manifest
//	.forge/specs/**
//
// `scratch` patterns mark deletion candidates. `managed` patterns are
// allow-listed: a file matching any managed pattern is never proposed for
// deletion even if it also matches a scratch pattern (false-positive guard).
package manifest

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/teragrid/forge/internal/errcode"
)

// Reserved error codes.
var (
	ErrParse   = errcode.Register(errcode.Code(2300), "manifest parse error")
	ErrMissing = errcode.Register(errcode.Code(2301), "manifest file missing")
)

// File represents a parsed manifest.
type File struct {
	Path    string
	Scratch []string // glob patterns (doublestar-style)
	Managed []string
}

// DefaultPath is the conventional location relative to a project root.
const DefaultPath = ".forge/manifest"

// Default returns the manifest used when a project has none. The MVP default
// targets the LLM-scratch patterns from Spec §4 hygiene that affect every
// project regardless of stack.
func Default() File {
	return File{
		Path: "<built-in default>",
		Scratch: []string{
			"**/.forge/scratch/**",
			"**/.forge/cache/**",
			"**/*.tmp.*",
			"**/_scratch_*",
			"**/_llm_*",
			"**/*.llm.bak",
			"**/.aider*",
			"**/.cursorignore.bak",
		},
		Managed: []string{
			".gitignore",
			".gitleaks.toml",
			".forge/manifest",
			".forge/specs/**",
			"README.md",
			"LICENSE",
		},
	}
}

// Load reads a manifest from path. If the file is missing, returns Default()
// with the original path stamped on it (so `--explain` still names the source).
func Load(path string) (File, error) {
	f, err := os.Open(path) //nolint:gosec // path is user-controlled CLI input by design.
	if err != nil {
		if os.IsNotExist(err) {
			d := Default()
			d.Path = path + " (defaults; file not present)"
			return d, nil
		}
		return File{}, errcode.New(ErrMissing, "open manifest", err)
	}
	defer f.Close()
	return parse(path, f)
}

func parse(path string, r io.Reader) (File, error) {
	out := File{Path: path}
	section := ""
	sc := bufio.NewScanner(r)
	line := 0
	for sc.Scan() {
		line++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
			section = strings.ToLower(strings.TrimSpace(raw[1 : len(raw)-1]))
			switch section {
			case "scratch", "managed":
			default:
				return File{}, errcode.Newf(ErrParse, nil,
					"%s:%d unknown section %q (want [scratch] or [managed])", path, line, section)
			}
			continue
		}
		switch section {
		case "scratch":
			out.Scratch = append(out.Scratch, raw)
		case "managed":
			out.Managed = append(out.Managed, raw)
		default:
			return File{}, errcode.Newf(ErrParse, nil,
				"%s:%d pattern outside any section: %q", path, line, raw)
		}
	}
	if err := sc.Err(); err != nil {
		return File{}, errcode.New(ErrParse, fmt.Sprintf("scan %s", path), err)
	}
	return out, nil
}

// Match reports whether rel (a forward-slash relative path) matches any
// pattern in pats. Supports `**` (any depth) and `*` (single segment).
func Match(pats []string, rel string) bool {
	rel = filepath.ToSlash(rel)
	for _, p := range pats {
		if matchGlob(p, rel) {
			return true
		}
	}
	return false
}

// IsScratch returns true if rel is a deletion candidate per the manifest:
// matches a scratch pattern AND does not match any managed pattern.
func (m File) IsScratch(rel string) bool {
	if Match(m.Managed, rel) {
		return false
	}
	return Match(m.Scratch, rel)
}

// matchGlob is a tiny `**`-aware glob matcher. Behaviour:
//   - `**`           matches any number of path segments (including zero).
//   - `*`            matches anything within a single segment (no `/`).
//   - `?`            matches exactly one non-`/` rune.
//   - everything else matches literally.
//
// Comparison is case-sensitive (POSIX); Windows-callers should normalise to
// lowercase if needed (we don't, to stay correct on case-sensitive filesystems
// shared via Docker etc.).
func matchGlob(pattern, name string) bool {
	pp := strings.Split(pattern, "/")
	np := strings.Split(name, "/")
	return matchSegments(pp, np)
}

func matchSegments(pat, name []string) bool {
	for len(pat) > 0 {
		seg := pat[0]
		if seg == "**" {
			// Collapse consecutive `**`.
			for len(pat) > 1 && pat[1] == "**" {
				pat = pat[1:]
			}
			rest := pat[1:]
			if len(rest) == 0 {
				return true
			}
			for i := 0; i <= len(name); i++ {
				if matchSegments(rest, name[i:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		if !matchSingleSegment(seg, name[0]) {
			return false
		}
		pat = pat[1:]
		name = name[1:]
	}
	return len(name) == 0
}

func matchSingleSegment(pat, name string) bool {
	// `path/filepath`'s Match handles `*`/`?` correctly within a single segment
	// because the strings have no `/`.
	ok, err := filepath.Match(pat, name)
	if err != nil {
		return false
	}
	return ok
}
