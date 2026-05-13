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

// TEST-31: Resilience-invariant test.

package tasktests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/teragrid/forge/internal/resilience"
)

var errFakeDownstream = errors.New("downstream error")

// TC-31-01 (happy): a circuit breaker starts closed and allows calls.
func TestTC3101_CircuitBreakerInitiallyClosed(t *testing.T) {
	t.Parallel()
	cb := resilience.NewCircuitBreaker(resilience.CBConfig{Threshold: 5})
	if cb.State() != resilience.StateClosed {
		t.Errorf("initial state = %v, want StateClosed", cb.State())
	}
	// A successful call should remain closed.
	ctx := context.Background()
	result, err := cb.Do(ctx, func(_ context.Context) (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Errorf("Do returned error: %v", err)
	}
	if result != "ok" {
		t.Errorf("result = %q, want %q", result, "ok")
	}
	if cb.State() != resilience.StateClosed {
		t.Errorf("state after success = %v, want StateClosed", cb.State())
	}
}

// TC-31-02 (negative): after Threshold failures the circuit opens.
func TestTC3102_CircuitBreakerOpensAfterThreshold(t *testing.T) {
	t.Parallel()
	threshold := 3
	cb := resilience.NewCircuitBreaker(resilience.CBConfig{
		Threshold:    threshold,
		OpenDuration: 10 * time.Minute, // long so it won't close during test
	})
	ctx := context.Background()
	for i := 0; i < threshold; i++ {
		_, _ = cb.Do(ctx, func(_ context.Context) (string, error) {
			return "", errFakeDownstream
		})
	}
	if cb.State() != resilience.StateOpen {
		t.Errorf("state after %d failures = %v, want StateOpen", threshold, cb.State())
	}
	// Next call should be rejected with ErrCircuitOpen.
	_, err := cb.Do(ctx, func(_ context.Context) (string, error) {
		return "should not be called", nil
	})
	if !errors.Is(err, resilience.ErrCircuitOpen) {
		t.Errorf("open CB: err = %v, want ErrCircuitOpen", err)
	}
}

// TC-31-03 (boundary): threshold = 1 opens on the very first failure.
func TestTC3103_CircuitBreakerThresholdOne(t *testing.T) {
	t.Parallel()
	cb := resilience.NewCircuitBreaker(resilience.CBConfig{
		Threshold:    1,
		OpenDuration: 10 * time.Minute,
	})
	ctx := context.Background()
	_, _ = cb.Do(ctx, func(_ context.Context) (string, error) {
		return "", errFakeDownstream
	})
	if cb.State() != resilience.StateOpen {
		t.Errorf("state after 1 failure with threshold=1 = %v, want StateOpen", cb.State())
	}
}

// TC-31-04 (idempotency): two separate circuit breakers are independent.
func TestTC3104_CircuitBreakerIndependence(t *testing.T) {
	t.Parallel()
	cb1 := resilience.NewCircuitBreaker(resilience.CBConfig{Threshold: 1, OpenDuration: 10 * time.Minute})
	cb2 := resilience.NewCircuitBreaker(resilience.CBConfig{Threshold: 5})
	ctx := context.Background()
	// Trip cb1.
	_, _ = cb1.Do(ctx, func(_ context.Context) (string, error) {
		return "", errFakeDownstream
	})
	if cb1.State() != resilience.StateOpen {
		t.Errorf("cb1 should be open")
	}
	// cb2 should still be closed.
	if cb2.State() != resilience.StateClosed {
		t.Errorf("cb2 state = %v, want StateClosed (should be independent)", cb2.State())
	}
}

// TC-31-07 (happy): a successful call through bulkhead completes normally.
func TestTC3107_BulkheadAllowsCall(t *testing.T) {
	t.Parallel()
	bh := resilience.NewBulkhead("test-bh", 5, 10)
	ctx := context.Background()
	result, err := bh.Do(ctx, func(_ context.Context) (string, error) {
		return "bulkhead-ok", nil
	})
	if err != nil {
		t.Errorf("bulkhead Do: %v", err)
	}
	if result != "bulkhead-ok" {
		t.Errorf("result = %q, want bulkhead-ok", result)
	}
}

// TC-31-08 (negative): Retry exhausts attempts and returns ErrMaxAttemptsExceeded.
func TestTC3108_RetryExhaustedReturnsTypedError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, err := resilience.Retry[string](ctx, resilience.RetryConfig{
		MaxAttempts:  2,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
	}, func(_ context.Context) (string, error) {
		return "", errFakeDownstream
	})
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	var maxErr *resilience.ErrMaxAttemptsExceeded
	if !errors.As(err, &maxErr) {
		t.Errorf("error type = %T, want *resilience.ErrMaxAttemptsExceeded", err)
	}
	if maxErr.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", maxErr.Attempts)
	}
}
