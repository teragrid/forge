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

// checkpointOrder defines the canonical forge ship pipeline stages in order.
var checkpointOrder = []string{"spec", "arch", "test", "breakdown", "code", "ship", "verify"}

// nextCheckpoint returns the checkpoint name that follows the given one, or ""
// if the given checkpoint is the last one or not found.
func nextCheckpoint(current string) string {
	for i, cp := range checkpointOrder {
		if cp == current && i+1 < len(checkpointOrder) {
			return checkpointOrder[i+1]
		}
	}
	return ""
}

// NextActions returns the ordered list of concrete copy-pasteable commands an
// LLM should execute next after a forge checkpoint completes or fails.
//
//   - checkpoint: the checkpoint that just ran (e.g. "code").
//   - slug: the feature --name slug (e.g. "auth-email"); may be empty.
//   - failed: whether the checkpoint ended in failure.
func NextActions(checkpoint, slug string, failed bool) []string {
	var actions []string

	if failed {
		// On failure: fix the error, then re-run the same checkpoint.
		actions = append(actions, "forge doctor  # diagnose environment issues")
		if slug != "" {
			actions = append(actions, "forge ship "+checkpoint+" --name "+slug+"  # retry after fix")
		} else {
			actions = append(actions, "forge ship "+checkpoint+"  # retry after fix")
		}
		return actions
	}

	// On success: advance to next checkpoint.
	next := nextCheckpoint(checkpoint)
	if next != "" {
		if slug != "" {
			actions = append(actions, "forge ship "+next+" --name "+slug)
		} else {
			actions = append(actions, "forge ship "+next)
		}
	}

	// Checkpoint-specific guidance.
	switch checkpoint {
	case "spec":
		actions = append(actions, "review .forge/specs/"+slug+"/spec.md before proceeding")
	case "arch":
		actions = append(actions, "review .forge/specs/"+slug+"/arch.md before proceeding")
	case "breakdown":
		actions = append(actions, "review .forge/specs/"+slug+"/breakdown.md; adjust task count if needed")
	case "code":
		actions = append(actions, "go test ./... -count=1  # verify test suite is green")
	case "ship":
		actions = append(actions, "forge ship verify --name "+slug+"  # run final quality gate")
	case "verify":
		actions = append(actions, "git push origin HEAD  # push to remote and open PR")
	}

	return actions
}
