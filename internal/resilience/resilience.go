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

// Package resilience implements DEV-M1-39: foundation-layer resilience
// primitives for Forge and adopted projects.
//
// Primitives shipped:
//   - CircuitBreaker — half-open probe; open-state short-circuit
//   - RetryWithJitter — exponential backoff with ±jitter
//   - Bulkhead — cap concurrent calls per named resource
//   - TimeoutBudget — per-call deadline with context propagation
//
// Usage example:
//
//	cb := resilience.NewCircuitBreaker(resilience.CBConfig{Threshold: 5, HalfOpenProbes: 1})
//	bh := resilience.NewBulkhead("llm", 10, 50)
//	result, err := resilience.Retry(ctx, resilience.RetryConfig{MaxAttempts: 3}, func(ctx context.Context) (string, error) {
//	    return bh.Do(ctx, func(ctx context.Context) (string, error) {
//	        return cb.Do(ctx, func(ctx context.Context) (string, error) {
//	            return callLLM(ctx)
//	        })
//	    })
//	})
package resilience

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// ── Errors ────────────────────────────────────────────────────────────────────

// ErrCircuitOpen is returned when the circuit breaker is in the open state.
var ErrCircuitOpen = errors.New("resilience: circuit breaker open")

// ErrBulkheadFull is returned when the bulkhead queue is at capacity.
var ErrBulkheadFull = errors.New("resilience: bulkhead at capacity")

// ErrMaxAttemptsExceeded is returned when all retry attempts are exhausted.
type ErrMaxAttemptsExceeded struct {
	Attempts int
	Last     error
}

func (e *ErrMaxAttemptsExceeded) Error() string {
	return fmt.Sprintf("resilience: %d attempts exhausted, last error: %v", e.Attempts, e.Last)
}

func (e *ErrMaxAttemptsExceeded) Unwrap() error { return e.Last }

// ── CircuitBreaker ────────────────────────────────────────────────────────────

// State is the circuit breaker state.
type State int32

const (
	StateClosed   State = 0 // normal operation
	StateOpen     State = 1 // rejecting calls
	StateHalfOpen State = 2 // probing one call
)

// CBConfig configures a circuit breaker.
type CBConfig struct {
	// Threshold is the number of consecutive failures before opening.
	Threshold int

	// OpenDuration is how long to stay open before moving to half-open.
	// Default: 10 seconds.
	OpenDuration time.Duration

	// HalfOpenProbes is the number of successful probes needed to close.
	// Default: 1.
	HalfOpenProbes int
}

// CircuitBreaker wraps calls to protect a downstream resource.
type CircuitBreaker struct {
	cfg          CBConfig
	state        atomic.Int32 // State
	failures     atomic.Int32
	probeSuccess atomic.Int32
	openAt       atomic.Int64 // unix nano
	mu           sync.Mutex
}

// NewCircuitBreaker creates a CircuitBreaker with the given config.
func NewCircuitBreaker(cfg CBConfig) *CircuitBreaker {
	if cfg.OpenDuration == 0 {
		cfg.OpenDuration = 10 * time.Second
	}
	if cfg.HalfOpenProbes == 0 {
		cfg.HalfOpenProbes = 1
	}
	if cfg.Threshold == 0 {
		cfg.Threshold = 5
	}
	return &CircuitBreaker{cfg: cfg}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() State {
	s := State(cb.state.Load())
	if s == StateOpen {
		openAt := time.Unix(0, cb.openAt.Load())
		if time.Since(openAt) >= cb.cfg.OpenDuration {
			cb.state.CompareAndSwap(int32(StateOpen), int32(StateHalfOpen))
			cb.probeSuccess.Store(0)
			return StateHalfOpen
		}
	}
	return s
}

// Do executes fn through the circuit breaker. Returns ErrCircuitOpen when open.
func (cb *CircuitBreaker) Do(ctx context.Context, fn func(context.Context) (string, error)) (string, error) {
	switch cb.State() {
	case StateOpen:
		return "", ErrCircuitOpen
	case StateHalfOpen:
		// Allow exactly one probe through; others fail fast.
		cb.mu.Lock()
		if cb.State() != StateHalfOpen {
			cb.mu.Unlock()
			return cb.Do(ctx, fn)
		}
		cb.mu.Unlock()
	}

	result, err := fn(ctx)
	if err != nil {
		cb.recordFailure()
		return "", err
	}
	cb.recordSuccess()
	return result, nil
}

func (cb *CircuitBreaker) recordFailure() {
	n := cb.failures.Add(1)
	if int(n) >= cb.cfg.Threshold {
		if cb.state.CompareAndSwap(int32(StateClosed), int32(StateOpen)) ||
			cb.state.CompareAndSwap(int32(StateHalfOpen), int32(StateOpen)) {
			cb.openAt.Store(time.Now().UnixNano())
			cb.failures.Store(0)
		}
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	switch cb.State() {
	case StateHalfOpen:
		n := cb.probeSuccess.Add(1)
		if int(n) >= cb.cfg.HalfOpenProbes {
			cb.state.Store(int32(StateClosed))
			cb.failures.Store(0)
		}
	case StateClosed:
		cb.failures.Store(0)
	}
}

// ── RetryWithJitter ───────────────────────────────────────────────────────────

// RetryConfig configures the retry policy.
type RetryConfig struct {
	// MaxAttempts is the maximum number of total attempts (including the first).
	MaxAttempts int

	// InitialDelay is the delay before the second attempt.
	// Default: 100ms.
	InitialDelay time.Duration

	// MaxDelay caps the computed backoff delay.
	// Default: 30s.
	MaxDelay time.Duration

	// JitterFraction is the random ± fraction applied to each delay (0..1).
	// Default: 0.25 (25%).
	JitterFraction float64

	// Retryable, if non-nil, is called with each error to determine whether
	// to retry. When nil all errors are retried.
	Retryable func(error) bool
}

// Retry executes fn with exponential backoff and jitter.
// On success it returns (result, nil).
// After MaxAttempts failures it returns ("", *ErrMaxAttemptsExceeded).
func Retry[T any](ctx context.Context, cfg RetryConfig, fn func(context.Context) (T, error)) (T, error) {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.InitialDelay == 0 {
		cfg.InitialDelay = 100 * time.Millisecond
	}
	if cfg.MaxDelay == 0 {
		cfg.MaxDelay = 30 * time.Second
	}
	if cfg.JitterFraction == 0 {
		cfg.JitterFraction = 0.25
	}

	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			var zero T
			return zero, ctx.Err()
		}
		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if cfg.Retryable != nil && !cfg.Retryable(err) {
			break
		}
		if attempt < cfg.MaxAttempts-1 {
			jitter := time.Duration(float64(delay) * cfg.JitterFraction * (rand.Float64()*2 - 1)) //nolint:gosec
			sleep := delay + jitter
			if sleep < 0 {
				sleep = 0
			}
			if sleep > cfg.MaxDelay {
				sleep = cfg.MaxDelay
			}
			select {
			case <-ctx.Done():
				var zero T
				return zero, ctx.Err()
			case <-time.After(sleep):
			}
			delay *= 2
			if delay > cfg.MaxDelay {
				delay = cfg.MaxDelay
			}
		}
	}
	var zero T
	return zero, &ErrMaxAttemptsExceeded{Attempts: cfg.MaxAttempts, Last: lastErr}
}

// ── Bulkhead ──────────────────────────────────────────────────────────────────

// Bulkhead caps the number of concurrent calls to a resource.
type Bulkhead struct {
	name     string
	sem      chan struct{}
	queueCap int
}

// NewBulkhead creates a Bulkhead limiting concurrency to maxConcurrent and the
// wait queue to queueCap (0 = no queuing; fail-fast when at capacity).
func NewBulkhead(name string, maxConcurrent, queueCap int) *Bulkhead {
	total := maxConcurrent + queueCap
	if total <= 0 {
		total = 1
	}
	return &Bulkhead{
		name:     name,
		sem:      make(chan struct{}, total),
		queueCap: queueCap,
	}
}

// Do executes fn within the bulkhead. Returns ErrBulkheadFull if the queue
// is full. Blocks when the concurrent slot is taken but queue space exists.
func (b *Bulkhead) Do(ctx context.Context, fn func(context.Context) (string, error)) (string, error) {
	select {
	case b.sem <- struct{}{}:
		defer func() { <-b.sem }()
		return fn(ctx)
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		return "", fmt.Errorf("%w: resource=%s", ErrBulkheadFull, b.name)
	}
}

// ── TimeoutBudget ─────────────────────────────────────────────────────────────

// WithTimeout runs fn within a deadline. If fn doesn't complete in time,
// the context is cancelled and the deadline-exceeded error is returned.
func WithTimeout[T any](ctx context.Context, d time.Duration, fn func(context.Context) (T, error)) (T, error) {
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	return fn(ctx)
}
