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

package llmresponse

import (
	"os"
)

// DetectOptions configure the LLM mode auto-detection.
type DetectOptions struct {
	// JSONFlag is true when --json was passed on the command line.
	JSONFlag bool
	// HumanFlag is true when --human was passed (opt-out, overrides everything).
	HumanFlag bool
	// Stdout is the writer to probe for TTY; defaults to os.Stdout when nil.
	Stdout *os.File
}

// DetectMode applies the priority chain defined in the LLM-first arch spec:
//
//  1. --human flag → human mode (false) — explicit opt-out always wins
//  2. --json flag → LLM mode (true)
//  3. FORGE_LLM_MODE=1 → LLM mode (true)
//  4. NO_COLOR=1 → LLM mode (true) — common CI signal
//  5. stdout is not a TTY → LLM mode (true)
//  6. default → human mode (false)
func DetectMode(opts DetectOptions) bool {
	// Rule 1: explicit opt-out always wins.
	if opts.HumanFlag {
		return false
	}
	// Rule 2: explicit machine flag.
	if opts.JSONFlag {
		return true
	}
	// Rule 3: environment override.
	if os.Getenv("FORGE_LLM_MODE") == "1" {
		return true
	}
	// Rule 4: standard CI "no colour" signal.
	if os.Getenv("NO_COLOR") == "1" {
		return true
	}
	// Rule 5: stdout is not a terminal (piped / redirected).
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	if fi, err := stdout.Stat(); err == nil {
		if fi.Mode()&os.ModeCharDevice == 0 {
			return true
		}
	}
	return false
}
