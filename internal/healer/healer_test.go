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

package healer_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/healer"
)

var errMissing = errors.New("forge.yaml not found")
var errOther = errors.New("unrelated error")

func matchMissing(err error) bool {
	return strings.Contains(err.Error(), "forge.yaml")
}

func fixMissing(_ context.Context, _ error) (string, error) {
	return "created default forge.yaml", nil
}

// TestHealer_MatchesAndFixes verifies a matching hook fires and returns description.
func TestHealer_MatchesAndFixes(t *testing.T) {
	t.Parallel()
	h := healer.New()
	h.Register(healer.Hook{Name: "missing-config", Match: matchMissing, Fix: fixMissing})

	result, err := h.Heal(context.Background(), errMissing)
	if err != nil {
		t.Fatalf("Heal: %v", err)
	}
	if result == nil {
		t.Fatal("expected HealResult, got nil")
	}
	if result.HookName != "missing-config" {
		t.Errorf("HookName = %q, want %q", result.HookName, "missing-config")
	}
	if result.Description == "" {
		t.Error("Description is empty")
	}
	if result.Err != nil {
		t.Errorf("fix error: %v", result.Err)
	}
}

// TestHealer_NoMatchReturnsNil verifies unmatched errors return (nil, nil).
func TestHealer_NoMatchReturnsNil(t *testing.T) {
	t.Parallel()
	h := healer.New()
	h.Register(healer.Hook{Name: "missing-config", Match: matchMissing, Fix: fixMissing})

	result, err := h.Heal(context.Background(), errOther)
	if err != nil {
		t.Fatalf("Heal: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for unmatched error, got: %+v", result)
	}
}

// TestHealer_NilErrorReturnsNil is a no-op.
func TestHealer_NilErrorReturnsNil(t *testing.T) {
	t.Parallel()
	h := healer.New()
	result, err := h.Heal(context.Background(), nil)
	if err != nil {
		t.Fatalf("Heal(nil): %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for nil error, got: %+v", result)
	}
}

// TestHealer_FixFailed captures fix error in HealResult.Err.
func TestHealer_FixFailed(t *testing.T) {
	t.Parallel()
	fixErr := fmt.Errorf("cannot create file: permission denied")
	h := healer.New()
	h.Register(healer.Hook{
		Name:  "failing-fix",
		Match: matchMissing,
		Fix: func(_ context.Context, _ error) (string, error) {
			return "", fixErr
		},
	})
	result, err := h.Heal(context.Background(), errMissing)
	if err != nil {
		t.Fatalf("Heal: %v", err)
	}
	if result == nil {
		t.Fatal("expected HealResult")
	}
	if result.Err == nil {
		t.Error("expected fix error in HealResult.Err")
	}
}

// TestHealer_FirstMatchWins verifies only the first matching hook fires.
func TestHealer_FirstMatchWins(t *testing.T) {
	t.Parallel()
	called := make([]string, 0)
	h := healer.New()
	h.Register(healer.Hook{
		Name:  "first",
		Match: matchMissing,
		Fix: func(_ context.Context, _ error) (string, error) {
			called = append(called, "first")
			return "first fixed", nil
		},
	})
	h.Register(healer.Hook{
		Name:  "second",
		Match: matchMissing,
		Fix: func(_ context.Context, _ error) (string, error) {
			called = append(called, "second")
			return "second fixed", nil
		},
	})
	_, _ = h.Heal(context.Background(), errMissing)
	if len(called) != 1 || called[0] != "first" {
		t.Errorf("expected only first hook to fire, got: %v", called)
	}
}

// TestHealOrReturn_Healed returns nil when fix succeeds.
func TestHealOrReturn_Healed(t *testing.T) {
	t.Parallel()
	h := healer.New()
	h.Register(healer.Hook{Name: "config", Match: matchMissing, Fix: fixMissing})
	if err := h.HealOrReturn(context.Background(), errMissing); err != nil {
		t.Errorf("HealOrReturn: expected nil after healing, got: %v", err)
	}
}

// TestHealOrReturn_NoMatch returns original error.
func TestHealOrReturn_NoMatch(t *testing.T) {
	t.Parallel()
	h := healer.New()
	h.Register(healer.Hook{Name: "config", Match: matchMissing, Fix: fixMissing})
	if err := h.HealOrReturn(context.Background(), errOther); !errors.Is(err, errOther) {
		t.Errorf("HealOrReturn: expected original error, got: %v", err)
	}
}
