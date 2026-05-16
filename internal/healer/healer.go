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

// Package healer provides G-110 self-healing runtime hooks.
//
// The healer monitors error events emitted by forge verbs and attempts to
// apply a registered fix automatically. Each Hook is a (matcher, fixer) pair:
//
//   - Matcher returns true when the hook should fire for a given error.
//   - Fixer is a function that attempts to resolve the error and returns a
//     human-readable description of what it did (or an error if it failed).
//
// Usage:
//
//	h := healer.New()
//	h.Register(healer.Hook{
//	    Name: "missing-config",
//	    Match: func(err error) bool { return strings.Contains(err.Error(), "forge.yaml") },
//	    Fix:   func(ctx context.Context, err error) (string, error) {
//	        // create a default forge.yaml
//	        return "created default forge.yaml", os.WriteFile("forge.yaml", defaults, 0600)
//	    },
//	})
//	description, err := h.Heal(ctx, someErr)
package healer

import (
	"context"
	"fmt"
)

// Hook is a (matcher, fixer) pair.
type Hook struct {
	// Name is a short identifier for the hook (used in logs and reports).
	Name string
	// Match returns true if this hook should fire for the given error.
	Match func(err error) bool
	// Fix attempts to resolve the error and returns a description.
	Fix func(ctx context.Context, err error) (string, error)
}

// Healer holds a set of hooks and applies the first matching one.
type Healer struct {
	hooks []Hook
}

// New returns an empty Healer.
func New() *Healer {
	return &Healer{}
}

// Register appends a hook to the healer's chain.
func (h *Healer) Register(hook Hook) {
	h.hooks = append(h.hooks, hook)
}

// HealResult describes the outcome of a healing attempt.
type HealResult struct {
	// HookName is the hook that fired.
	HookName string
	// Description is what the hook did.
	Description string
	// Err is non-nil if the fix itself failed.
	Err error
}

// Heal walks the registered hooks and applies the first one whose Match
// returns true for err. Returns (nil, nil) if no hook matched (caller should
// handle the error normally).
func (h *Healer) Heal(ctx context.Context, err error) (*HealResult, error) {
	if err == nil {
		return nil, nil
	}
	for _, hook := range h.hooks {
		if hook.Match(err) {
			desc, fixErr := hook.Fix(ctx, err)
			return &HealResult{
				HookName:    hook.Name,
				Description: desc,
				Err:         fixErr,
			}, nil
		}
	}
	return nil, nil
}

// HealOrReturn is a convenience wrapper: if Heal finds a matching hook and the
// fix succeeds it returns nil (healed). Otherwise it returns the original err.
func (h *Healer) HealOrReturn(ctx context.Context, err error) error {
	result, healErr := h.Heal(ctx, err)
	if healErr != nil {
		return fmt.Errorf("healer internal error: %w", healErr)
	}
	if result == nil {
		// No hook matched.
		return err
	}
	if result.Err != nil {
		// Hook matched but fix failed.
		return fmt.Errorf("healer %q failed: %w (original: %v)", result.HookName, result.Err, err)
	}
	// Fix succeeded.
	return nil
}

// Len returns the number of registered hooks.
func (h *Healer) Len() int { return len(h.hooks) }
