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

// test_reachability.go — does anything actually RUN the tests forge just wrote?
//
// # The gap this closes
//
// The Test checkpoint wrote its artefacts, counted them, and reported green.
// What it never checked is whether any configured runner would execute them.
// Those are different questions, and on a real project they came apart:
//
//	jest.config.js              testPathIgnorePatterns: ["\\.(integration|e2e)\\."]
//	jest.integration.config.js  testMatch: ["<rootDir>/tests/**/integration/*.test.ts"]
//
// Forge writes `tests/<slug>.integration.test.ts`. The default config ignores
// it for having `.integration.` in the name; the integration config does not
// match it for not being under an `integration/` directory. The file exists,
// the checkpoint is green, and the test has never run once — not in CI, not
// locally, not ever. Coverage reports do not show it as uncovered, because a
// test that no runner collects is not a gap in the report; it is absent from
// the report's universe.
//
// That is worse than having no test. A missing test is visibly missing. A test
// that silently never runs is indistinguishable from a passing one.
//
// # How reachability is decided
//
// Three tiers, in descending order of trust, because the answer is only worth
// having if we are honest about where it came from:
//
//  1. **Ask the runner.** Jest and Vitest can both enumerate the files they
//     would collect (`jest --listTests`, `vitest list`). This is authoritative
//     — it is the resolver that will actually run in CI, not a reimplementation
//     of it. Only a locally installed runner is used; forge never resolves a
//     package from the network to answer a question about the working tree.
//  2. **Read the configs.** When no runner is installed, the config files are
//     scanned for `testPathIgnorePatterns`, `testMatch` and `testRegex`. This
//     catches the failure above, which is the common shape, but it cannot see
//     a config that computes its patterns at runtime.
//  3. **Say we don't know.** If neither works, the checkpoint reports the
//     artefacts as unverified rather than implying they are wired up. The
//     whole point of this file is to stop reporting unearned confidence, so it
//     must not invent any of its own.
package cmdship

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/teragrid/forge/internal/procspawn"
)

// ReachabilityMethod records how a verdict was reached, so callers can phrase
// the result with the confidence it actually warrants.
type ReachabilityMethod string

const (
	// MethodRunner — the project's own test runner enumerated the files.
	MethodRunner ReachabilityMethod = "runner"
	// MethodStatic — inferred by reading the runner's config files.
	MethodStatic ReachabilityMethod = "static"
	// MethodNone — could not be determined.
	MethodNone ReachabilityMethod = "none"
)

// ReachabilityReport is the outcome of checking a set of written test files.
type ReachabilityReport struct {
	Method ReachabilityMethod `json:"method"`
	// Orphans are files no configured runner would execute — repo-relative,
	// forward-slash, sorted for stable output.
	Orphans []string `json:"orphans,omitempty"`
	// Checked is the number of files examined.
	Checked int `json:"checked"`
	// Configs are the runner config files considered, repo-relative.
	Configs []string `json:"configs,omitempty"`
	// Note explains a MethodNone verdict, or any caveat on the others.
	Note string `json:"note,omitempty"`
}

// OK reports whether the report is free of orphans. A MethodNone report is
// never OK — "unknown" must not read as "fine" at the call site.
func (r ReachabilityReport) OK() bool {
	return r.Method != MethodNone && len(r.Orphans) == 0
}

// Summary renders the report as a one-line checkpoint detail fragment.
func (r ReachabilityReport) Summary() string {
	switch {
	case r.Method == MethodNone:
		return "test reachability unverified (" + r.Note + ") — confirm your runner actually collects tests/"
	case len(r.Orphans) == 0:
		if r.Method == MethodStatic {
			return "all test files are matched by a runner config (static check; no runner installed)"
		}
		return "all test files are collected by the project's test runner"
	default:
		verb := "no runner config matches"
		if r.Method == MethodRunner {
			verb = "the test runner does not collect"
		}
		return "UNREACHABLE TESTS: " + verb + " " + strings.Join(r.Orphans, ", ") +
			" — these file(s) will never run; rename them or widen the runner config"
	}
}

// verifyTestsReachable checks whether each file will be executed by any of the
// project's configured JS/TS test runners.
//
// files are absolute paths. Non-JS/TS projects return MethodNone with a note:
// `go test ./...` and pytest collect by convention rather than configuration,
// so there is no equivalent dead zone for a file in the tests directory to
// fall into, and inventing a check for one would only produce noise.
func verifyTestsReachable(root string, files []string) ReachabilityReport {
	rep := ReachabilityReport{Method: MethodNone}

	var rel []string
	for _, f := range files {
		if f == "" {
			continue
		}
		if !isJSTestFile(f) {
			continue
		}
		rel = append(rel, relSlash(root, f))
	}
	rep.Checked = len(rel)
	if len(rel) == 0 {
		rep.Note = "no JavaScript/TypeScript test files to check"
		return rep
	}

	configs := findJSTestConfigs(root)
	rep.Configs = configs

	// Tier 1 — ask the runner.
	if collected, cfgs, ok := listTestsViaRunner(root, configs); ok {
		rep.Method = MethodRunner
		rep.Configs = cfgs
		rep.Orphans = missingFrom(collected, rel)
		return rep
	}

	// Tier 2 — read the configs.
	if len(configs) > 0 {
		matchers, parsed := parseJSTestConfigs(root, configs)
		if parsed > 0 {
			rep.Method = MethodStatic
			for _, f := range rel {
				if !anyMatcherCollects(matchers, f) {
					rep.Orphans = append(rep.Orphans, f)
				}
			}
			return rep
		}
	}

	// Tier 3 — say we don't know.
	if len(configs) == 0 {
		rep.Note = "no jest/vitest config found"
	} else {
		rep.Note = "runner not installed and config patterns are computed at runtime"
	}
	return rep
}

// ── Tier 1: ask the runner ────────────────────────────────────────────────────

// listTestsViaRunner enumerates collected test files using the locally
// installed runner, once per config (a split unit/integration setup only makes
// sense as the union of its configs).
//
// Only node_modules/.bin is consulted. Resolving the runner from the network
// would make a checkpoint depend on registry availability and could install a
// package as a side effect of a read-only question.
func listTestsViaRunner(root string, configs []string) (collected map[string]bool, used []string, ok bool) {
	collected = map[string]bool{}
	runner, name := localJSRunner(root)
	if runner == "" {
		return nil, nil, false
	}

	sp := procspawn.New(runner)
	run := func(args []string) (string, bool) {
		res, err := sp.Run(runner, args, procspawn.Options{
			Dir:     root,
			Timeout: 90 * time.Second,
		})
		if err != nil || res == nil || res.ExitCode != 0 {
			return "", false
		}
		return res.Stdout, true
	}

	// Jest prints one absolute path per line for --listTests; Vitest's
	// `list` does the same. Both accept an explicit --config.
	attempt := func(cfg string) bool {
		var args []string
		switch name {
		case "vitest":
			args = []string{"list"}
			if cfg != "" {
				args = append(args, "--config", cfg)
			}
		default: // jest
			args = []string{"--listTests"}
			if cfg != "" {
				args = append(args, "--config", cfg)
			}
		}
		out, good := run(args)
		if !good {
			return false
		}
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || !isJSTestFile(line) {
				continue
			}
			collected[relSlash(root, line)] = true
		}
		return true
	}

	if len(configs) == 0 {
		if attempt("") {
			return collected, nil, true
		}
		return nil, nil, false
	}
	for _, cfg := range configs {
		if attempt(filepath.Join(root, filepath.FromSlash(cfg))) {
			used = append(used, cfg)
		}
	}
	if len(used) == 0 {
		return nil, nil, false
	}
	return collected, used, true
}

// localJSRunner returns the path to a locally installed jest or vitest binary.
func localJSRunner(root string) (path, name string) {
	bin := filepath.Join(root, "node_modules", ".bin")
	for _, n := range []string{"jest", "vitest"} {
		for _, ext := range binExtensions() {
			p := filepath.Join(bin, n+ext)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p, n
			}
		}
	}
	return "", ""
}

// ── Tier 2: read the configs ──────────────────────────────────────────────────

// configMatcher is one runner config reduced to the three rules that decide
// whether it collects a given path.
type configMatcher struct {
	name    string
	ignore  []*regexp.Regexp // testPathIgnorePatterns — any match excludes
	match   []*regexp.Regexp // testMatch globs, compiled — any match includes
	regexps []*regexp.Regexp // testRegex — any match includes
}

// collects reports whether this config would run the repo-relative path.
func (m configMatcher) collects(rel string) bool {
	for _, re := range m.ignore {
		if re.MatchString(rel) {
			return false
		}
	}
	if len(m.match) == 0 && len(m.regexps) == 0 {
		// No explicit include rules: the runner's own default testMatch
		// applies, which collects **/*.{test,spec}.{js,ts,jsx,tsx} anywhere.
		return defaultJSTestPattern.MatchString(rel)
	}
	for _, re := range m.match {
		if re.MatchString(rel) {
			return true
		}
	}
	for _, re := range m.regexps {
		if re.MatchString(rel) {
			return true
		}
	}
	return false
}

func anyMatcherCollects(ms []configMatcher, rel string) bool {
	for _, m := range ms {
		if m.collects(rel) {
			return true
		}
	}
	return false
}

// defaultJSTestPattern mirrors Jest's and Vitest's default collection rule.
var defaultJSTestPattern = regexp.MustCompile(`(^|/)(__tests__/.*|.*\.(test|spec))\.[cm]?[jt]sx?$`)

// arrayFieldRe extracts a JS/JSON array-of-strings field by name. It matches
// the literal-array form only — a config that builds its patterns from a
// variable yields no match, and the caller degrades to MethodNone rather than
// guessing.
func arrayFieldRe(field string) *regexp.Regexp {
	return regexp.MustCompile(`["']?` + field + `["']?\s*:\s*\[([^\]]*)\]`)
}

var stringLiteralRe = regexp.MustCompile(`["'` + "`" + `]([^"'` + "`" + `]+)["'` + "`" + `]`)

// parseJSTestConfigs reduces each config file to a configMatcher. It returns
// how many configs yielded at least one usable rule, so the caller can tell
// "parsed and found nothing wrong" from "could not parse anything".
func parseJSTestConfigs(root string, configs []string) (matchers []configMatcher, parsed int) {
	for _, cfg := range configs {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(cfg)))
		if err != nil {
			continue
		}
		body := string(data)
		if cfg == "package.json" {
			body = extractPackageJSONJest(data)
			if body == "" {
				continue
			}
		}
		m := configMatcher{name: cfg}
		var usable bool

		for _, lit := range arrayLiterals(body, "testPathIgnorePatterns") {
			if re, err := regexp.Compile(lit); err == nil {
				m.ignore = append(m.ignore, re)
				usable = true
			}
		}
		for _, lit := range arrayLiterals(body, "testMatch") {
			if re, err := regexp.Compile(globToRegexp(stripRootDir(lit))); err == nil {
				m.match = append(m.match, re)
				usable = true
			}
		}
		for _, lit := range arrayLiterals(body, "testRegex") {
			if re, err := regexp.Compile(lit); err == nil {
				m.regexps = append(m.regexps, re)
				usable = true
			}
		}
		if usable {
			parsed++
		}
		matchers = append(matchers, m)
	}
	return matchers, parsed
}

// arrayLiterals returns the string literals inside the named array field.
func arrayLiterals(body, field string) []string {
	loc := arrayFieldRe(field).FindStringSubmatch(body)
	if len(loc) < 2 {
		return nil
	}
	var out []string
	for _, m := range stringLiteralRe.FindAllStringSubmatch(loc[1], -1) {
		if s := strings.TrimSpace(unescapeStringLiteral(m[1])); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// unescapeStringLiteral resolves the backslash escapes in a JS or JSON string
// literal so the result is the value the runner sees, not the source text.
//
// This is not cosmetic. These fields hold regexes, so they are double-escaped
// at the source level: `testPathIgnorePatterns: ['\\.(integration|e2e)\\.']`
// is the JS spelling of the regex `\.(integration|e2e)\.`. Compiling the raw
// source text instead yields `\\.` — "a literal backslash, then any character"
// — which matches nothing in a real path. The checker would then find every
// ignore rule inert, conclude nothing was excluded, and report the dead zone
// as reachable: a false green from the very check written to stop false greens.
func unescapeStringLiteral(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			sb.WriteByte(s[i])
			continue
		}
		// Only the escapes that change meaning here are resolved. A regex
		// escape such as `\d` or `\.` must survive intact — consuming its
		// backslash would corrupt the pattern just as badly as leaving the
		// doubled one in place.
		switch s[i+1] {
		case '\\', '/', '\'', '"', '`':
			sb.WriteByte(s[i+1])
			i++
		default:
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}

// extractPackageJSONJest returns the "jest" object of a package.json as text.
func extractPackageJSONJest(data []byte) string {
	var pkg map[string]json.RawMessage
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	raw, ok := pkg["jest"]
	if !ok {
		return ""
	}
	return string(raw)
}

// stripRootDir removes Jest's <rootDir> token, which is always the directory
// paths are already relative to here.
func stripRootDir(s string) string {
	s = strings.ReplaceAll(s, "<rootDir>/", "")
	return strings.ReplaceAll(s, "<rootDir>", "")
}

// globToRegexp converts the glob subset Jest's testMatch actually uses into a
// regexp anchored at both ends.
//
// Order matters: `**/` is consumed before `*`, otherwise the single-star rule
// would eat the globstar and `**/foo` would stop matching nested paths — the
// precise mistake that would make this checker report false orphans.
func globToRegexp(glob string) string {
	var sb strings.Builder
	sb.WriteString("^")
	for i := 0; i < len(glob); i++ {
		switch {
		case strings.HasPrefix(glob[i:], "**/"):
			sb.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(glob[i:], "**"):
			sb.WriteString(".*")
			i++
		case glob[i] == '*':
			sb.WriteString("[^/]*")
		case glob[i] == '?':
			sb.WriteString("[^/]")
		case glob[i] == '{':
			if end := strings.IndexByte(glob[i:], '}'); end > 0 {
				alts := strings.Split(glob[i+1:i+end], ",")
				for j, a := range alts {
					alts[j] = regexp.QuoteMeta(a)
				}
				sb.WriteString("(?:" + strings.Join(alts, "|") + ")")
				i += end
				continue
			}
			sb.WriteString(regexp.QuoteMeta(string(glob[i])))
		default:
			sb.WriteString(regexp.QuoteMeta(string(glob[i])))
		}
	}
	sb.WriteString("$")
	return sb.String()
}

// ── shared helpers ────────────────────────────────────────────────────────────

// findJSTestConfigs lists the runner config files present at the project root,
// repo-relative and forward-slashed. A split setup (jest.config.js plus
// jest.integration.config.js) is the normal case, not an edge case — the union
// of every config is what decides whether a file runs.
func findJSTestConfigs(root string) []string {
	var out []string
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if isRunnerConfigName(n) {
			out = append(out, n)
		}
	}
	// package.json carries an inline "jest" key often enough to matter.
	if data, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		if extractPackageJSONJest(data) != "" {
			out = append(out, "package.json")
		}
	}
	return out
}

func isRunnerConfigName(n string) bool {
	lower := strings.ToLower(n)
	if !strings.HasPrefix(lower, "jest.") && !strings.HasPrefix(lower, "vitest.") {
		return false
	}
	if !strings.Contains(lower, ".config.") && !strings.HasSuffix(lower, ".config") {
		return false
	}
	for _, ext := range []string{".js", ".cjs", ".mjs", ".ts", ".mts", ".cts", ".json"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// binExtensions lists the suffixes a node_modules/.bin entry can carry on this
// OS. npm writes .cmd/.ps1 shims on Windows alongside the extensionless shell
// script; checking only the bare name would make every Windows project fall
// through to the weaker static tier.
func binExtensions() []string {
	if runtime.GOOS == "windows" {
		return []string{".cmd", ".CMD", ".exe", ""}
	}
	return []string{""}
}

func isJSTestFile(p string) bool {
	lower := strings.ToLower(filepath.ToSlash(p))
	for _, ext := range []string{".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// relSlash renders p relative to root with forward slashes, falling back to
// the base name when p lies outside root.
func relSlash(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return filepath.ToSlash(p)
}

// missingFrom returns the wanted entries absent from collected, preserving the
// caller's order so output is stable across runs.
func missingFrom(collected map[string]bool, wanted []string) []string {
	var out []string
	for _, w := range wanted {
		if !collected[w] {
			out = append(out, w)
		}
	}
	return out
}
