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

// code_validate.go — validation for LLM / host-agent answers that are SOURCE
// CODE (test files), as opposed to the Markdown artefacts artefact_validate.go
// checks. The Markdown checks are wrong for code: they cut everything before
// the first '#' line and require the answer to end on prose punctuation, so a
// real Jest suite ending in `});` was judged "truncated" and silently replaced
// by a red stub (field report ISSUE 13).
package cmdship

import "strings"

// validateCode extracts the code from raw and reports whether it looks whole.
// A fenced block, when present, is the code (any prose around it is dropped);
// otherwise the trimmed answer is. Complete means non-empty, no unterminated
// fence, and balanced (), {} and [] outside strings and comments.
func validateCode(raw, lang string) (code string, complete bool) {
	t := strings.TrimSpace(raw)
	if t == "" {
		return "", false
	}
	if i := strings.Index(t, "```"); i >= 0 {
		rest := t[i+3:]
		nl := strings.IndexByte(rest, '\n')
		if nl < 0 {
			return "", false
		}
		rest = rest[nl+1:]
		end := strings.Index(rest, "```")
		if end < 0 {
			return strings.TrimRight(rest, " \t\r\n") + "\n", false // unterminated fence
		}
		t = strings.TrimSpace(rest[:end])
		if t == "" {
			return "", false
		}
	}
	return t + "\n", delimitersBalanced(t, lang)
}

// delimitersBalanced scans code for (), {} and [] balance, skipping string
// literals and line/block comments. It is deliberately lenient: it only has
// to catch an answer cut off mid-function, not validate the grammar.
func delimitersBalanced(code, lang string) bool {
	hashComments := lang == "python"
	slashComments := lang != "python"
	depth := 0
	rs := []rune(code)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case slashComments && c == '/' && i+1 < len(rs) && rs[i+1] == '/',
			hashComments && c == '#':
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
		case slashComments && c == '/' && i+1 < len(rs) && rs[i+1] == '*':
			i += 2
			for i+1 < len(rs) && !(rs[i] == '*' && rs[i+1] == '/') {
				i++
			}
			i++
		case c == '"' || c == '\'' || c == '`':
			multiline := c == '`' || (lang == "python" && i+2 < len(rs) && rs[i+1] == c && rs[i+2] == c)
			if multiline && c != '`' {
				i += 3
				for i+2 < len(rs) && !(rs[i] == c && rs[i+1] == c && rs[i+2] == c) {
					i++
				}
				i += 2
				continue
			}
			for i++; i < len(rs); i++ {
				if rs[i] == '\\' {
					i++
					continue
				}
				if rs[i] == c || (!multiline && rs[i] == '\n') {
					break
				}
			}
		case c == '(' || c == '{' || c == '[':
			depth++
		case c == ')' || c == '}' || c == ']':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// generateCodeWithValidation is generateWithValidation for source-code
// answers: same retry-once and API-truncation semantics, code-aware checks.
func generateCodeWithValidation(lang string, invoke func() (string, bool, error)) (string, bool, error) {
	generated, apiTruncated, err := invoke()
	if err != nil {
		return "", false, err
	}
	if generated == "" {
		return "", false, nil
	}
	code, ok := validateCode(generated, lang)
	if ok && !apiTruncated {
		return code, true, nil
	}
	retry, retryTruncated, retryErr := invoke()
	if retryErr != nil || retry == "" {
		return code, false, nil
	}
	code2, ok2 := validateCode(retry, lang)
	return code2, ok2 && !retryTruncated, nil
}
