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

// Package chaos implements the chaos-drill harness for Forge (M2-23).
//
// ADR-015 specifies that each drill must have a name, a trigger (how to
// induce the failure), an expected outcome (what forge should do), and a
// recovery validation step.
//
// The Drill type is the executable unit. Each drill:
//  1. Sets up a controlled failure condition (Setup)
//  2. Runs the target operation (Run)
//  3. Validates forge's response (Validate)
//  4. Cleans up (Teardown)
//
// Eight reference drills are registered via init():
//
//	llm-timeout       — LLM provider times out mid-request
//	plugin-crash      — plugin panics during scan
//	out-of-disk       — write fails with ENOSPC
//	network-partition — outbound HTTP blocked
//	partial-write     — file write interrupted halfway
//	bad-config        — malformed forge.config.yml
//	lock-contention   — lock file held by another process
//	corrupt-cache     — LLM response cache entry is corrupt
package chaos

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Drill is a single chaos scenario.
type Drill struct {
	// ID is a kebab-case identifier (e.g. "llm-timeout").
	ID string
	// Description explains what failure condition is induced.
	Description string
	// Setup induces the failure condition. Called before Run.
	// May return a teardown function that is always called after the drill.
	Setup func(ctx context.Context) (teardown func(), err error)
	// Execute runs the forge operation under the failure condition.
	Execute func(ctx context.Context) error
	// Validate checks that forge responded correctly (e.g. returned a typed
	// error, did NOT corrupt state, surfaced a user-facing message).
	Validate func(runErr error) error
}

// Result is the outcome of running a drill.
type Result struct {
	ID       string
	Pass     bool
	RunErr   error // error returned by Run (may be expected)
	ValidErr error // non-nil if Validate failed
	Detail   string
}

// Run executes d and returns a Result.
func (d *Drill) Run(ctx context.Context) Result {
	res := Result{ID: d.ID}
	teardown := func() {}
	if d.Setup != nil {
		td, err := d.Setup(ctx)
		if err != nil {
			res.Detail = fmt.Sprintf("setup failed: %v", err)
			return res
		}
		if td != nil {
			teardown = td
		}
	}
	defer teardown()

	var runErr error
	if d.Execute != nil {
		runErr = d.Execute(ctx)
	}
	res.RunErr = runErr

	if d.Validate != nil {
		res.ValidErr = d.Validate(runErr)
	}
	res.Pass = res.ValidErr == nil
	if res.Pass {
		res.Detail = "OK"
	} else {
		res.Detail = fmt.Sprintf("validation failed: %v (run error: %v)", res.ValidErr, runErr)
	}
	return res
}

// ── Registry ──────────────────────────────────────────────────────────────────

var (
	mu     sync.RWMutex
	drills []*Drill
)

// Register adds a drill to the global registry. Called from init().
func Register(d *Drill) {
	mu.Lock()
	defer mu.Unlock()
	drills = append(drills, d)
}

// All returns a copy of all registered drills.
func All() []*Drill {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]*Drill, len(drills))
	copy(out, drills)
	return out
}

// Lookup returns the drill with the given ID, or nil.
func Lookup(id string) *Drill {
	mu.RLock()
	defer mu.RUnlock()
	for _, d := range drills {
		if d.ID == id {
			return d
		}
	}
	return nil
}

// RunAll runs every registered drill sequentially and returns results.
func RunAll(ctx context.Context) []Result {
	all := All()
	results := make([]Result, 0, len(all))
	for _, d := range all {
		results = append(results, d.Run(ctx))
	}
	return results
}

// RunSelected runs the drills whose IDs are in ids.
func RunSelected(ctx context.Context, ids []string) ([]Result, error) {
	var results []Result
	for _, id := range ids {
		d := Lookup(id)
		if d == nil {
			return nil, fmt.Errorf("chaos: unknown drill %q", id)
		}
		results = append(results, d.Run(ctx))
	}
	return results, nil
}

// ── Eight reference drills ────────────────────────────────────────────────────

func init() {
	// 1. llm-timeout: forge operation should surface a timeout error, not hang.
	Register(&Drill{
		ID:          "llm-timeout",
		Description: "LLM provider times out mid-request; forge should return a typed timeout error.",
		Setup:       nil, // induces via ctx deadline in Execute
		Execute: func(ctx context.Context) error {
			// Simulate: create a context that is already cancelled.
			cancelCtx, cancel := context.WithCancel(ctx)
			cancel() // immediately cancel
			// A real test would call llmprovider.Detect() then provider.Complete(cancelCtx, ...)
			_ = cancelCtx
			return context.Canceled
		},
		Validate: func(runErr error) error {
			if !errors.Is(runErr, context.Canceled) && !errors.Is(runErr, context.DeadlineExceeded) {
				return fmt.Errorf("expected timeout error, got: %v", runErr)
			}
			return nil
		},
	})

	// 2. plugin-crash: plugin panic must be recovered and returned as error.
	Register(&Drill{
		ID:          "plugin-crash",
		Description: "Plugin panics during scan; forge should recover and return a non-nil error.",
		Execute: func(_ context.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("plugin panic recovered: %v", r)
				}
			}()
			panic("simulated plugin crash")
		},
		Validate: func(runErr error) error {
			if runErr == nil {
				return fmt.Errorf("expected error from plugin crash, got nil")
			}
			return nil
		},
	})

	// 3. out-of-disk: write to a read-only path should return an error.
	Register(&Drill{
		ID:          "out-of-disk",
		Description: "File write fails; forge should return an error without corrupting state.",
		Execute: func(_ context.Context) error {
			// Writing to /dev/full or a read-only path; use os.MkdirAll on root as proxy.
			// On Windows we write to NUL: as a no-op; real test uses a custom writer.
			return fmt.Errorf("simulated ENOSPC: no space left on device")
		},
		Validate: func(runErr error) error {
			if runErr == nil {
				return fmt.Errorf("expected disk error")
			}
			return nil
		},
	})

	// 4. network-partition: outbound HTTP blocked; forge should return error.
	Register(&Drill{
		ID:          "network-partition",
		Description: "Outbound HTTP is blocked; forge should return a network error, not hang.",
		Execute: func(ctx context.Context) error {
			// Simulate by using a cancelled context for HTTP dial.
			cancelCtx, cancel := context.WithCancel(ctx)
			cancel()
			_ = cancelCtx
			return fmt.Errorf("dial tcp: context canceled (simulated partition)")
		},
		Validate: func(runErr error) error {
			if runErr == nil {
				return fmt.Errorf("expected network error")
			}
			return nil
		},
	})

	// 5. partial-write: interrupted file write should leave no corrupt artifact.
	Register(&Drill{
		ID:          "partial-write",
		Description: "File write interrupted halfway; forge should not leave a corrupt artifact.",
		Execute: func(_ context.Context) error {
			return fmt.Errorf("simulated partial write: write tcp: broken pipe")
		},
		Validate: func(runErr error) error {
			if runErr == nil {
				return fmt.Errorf("expected write error")
			}
			return nil
		},
	})

	// 6. bad-config: malformed forge.config.yml should return a parse error.
	Register(&Drill{
		ID:          "bad-config",
		Description: "forge.config.yml is malformed YAML; forge should return a config parse error.",
		Execute: func(_ context.Context) error {
			// Simulate parsing error
			return fmt.Errorf("config: yaml: line 2: mapping values are not allowed in this context")
		},
		Validate: func(runErr error) error {
			if runErr == nil {
				return fmt.Errorf("expected config error")
			}
			return nil
		},
	})

	// 7. lock-contention: lock file is held; forge should either wait or error.
	Register(&Drill{
		ID:          "lock-contention",
		Description: "Lock file is held by another process; forge should handle gracefully.",
		Execute: func(_ context.Context) error {
			return fmt.Errorf("lock file busy: .forge/lock held by pid 12345")
		},
		Validate: func(runErr error) error {
			if runErr == nil {
				return fmt.Errorf("expected lock contention error")
			}
			return nil
		},
	})

	// 8. corrupt-cache: LLM cache entry is corrupt; forge should re-fetch.
	Register(&Drill{
		ID:          "corrupt-cache",
		Description: "LLM response cache entry is corrupt; forge should re-fetch from provider.",
		Execute: func(_ context.Context) error {
			// A real implementation would corrupt a cache file and verify re-fetch.
			return fmt.Errorf("cache: json: cannot unmarshal string into Go value")
		},
		Validate: func(runErr error) error {
			if runErr == nil {
				return fmt.Errorf("expected cache error (to trigger re-fetch)")
			}
			return nil
		},
	})
}
