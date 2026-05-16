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

package llmprovider_test

import (
	"context"
	"testing"

	"github.com/teragrid/forge/internal/llmprovider"
)

// TestBatch_Empty returns nil for empty input.
func TestBatch_Empty(t *testing.T) {
	t.Parallel()
	m := &llmprovider.MockProvider{}
	results := llmprovider.Batch(context.Background(), m, nil, 0)
	if results != nil {
		t.Errorf("expected nil for empty batch, got %v", results)
	}
}

// TestBatch_SingleRequest processes one request.
func TestBatch_SingleRequest(t *testing.T) {
	t.Parallel()
	m := &llmprovider.MockProvider{Response: &llmprovider.Response{Content: "hello"}}
	reqs := []*llmprovider.Request{{UserPrompt: "ping"}}
	results := llmprovider.Batch(context.Background(), m, reqs, 1)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("unexpected error: %v", results[0].Err)
	}
	if results[0].Response == nil || results[0].Response.Content != "hello" {
		t.Errorf("unexpected content: %q", results[0].Response.Content)
	}
}

// TestBatch_MultipleRequests processes several requests, preserving order.
func TestBatch_MultipleRequests(t *testing.T) {
	t.Parallel()
	m := &llmprovider.MockProvider{Response: &llmprovider.Response{Content: "pong"}}
	n := 5
	reqs := make([]*llmprovider.Request, n)
	for i := range n {
		reqs[i] = &llmprovider.Request{UserPrompt: "q"}
	}
	results := llmprovider.Batch(context.Background(), m, reqs, 2)
	if len(results) != n {
		t.Fatalf("expected %d results, got %d", n, len(results))
	}
	for i, r := range results {
		if r.Index != i {
			t.Errorf("result[%d].Index = %d", i, r.Index)
		}
		if r.Err != nil {
			t.Errorf("result[%d].Err = %v", i, r.Err)
		}
	}
}

// TestBatch_ConcurrencyZero uses len(requests) workers.
func TestBatch_ConcurrencyZero(t *testing.T) {
	t.Parallel()
	m := &llmprovider.MockProvider{Response: &llmprovider.Response{Content: "ok"}}
	reqs := []*llmprovider.Request{{UserPrompt: "a"}, {UserPrompt: "b"}, {UserPrompt: "c"}}
	results := llmprovider.Batch(context.Background(), m, reqs, 0)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
}

// TestBatch_AllCallsAreMade counts provider invocations.
func TestBatch_AllCallsAreMade(t *testing.T) {
	t.Parallel()
	m := &llmprovider.MockProvider{Response: &llmprovider.Response{Content: "x"}}
	const count = 4
	reqs := make([]*llmprovider.Request, count)
	for i := range count {
		reqs[i] = &llmprovider.Request{UserPrompt: "test"}
	}
	llmprovider.Batch(context.Background(), m, reqs, 2)
	if m.Calls() != count {
		t.Errorf("expected %d calls, got %d", count, m.Calls())
	}
}
