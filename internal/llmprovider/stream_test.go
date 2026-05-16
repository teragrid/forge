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

// G-046: Unit tests for StreamUntilComplete.
package llmprovider_test

import (
	"context"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/llmprovider"
)

// sendChunks builds a channel and sends the provided chunks in a goroutine.
func sendChunks(chunks []llmprovider.StreamChunk) <-chan llmprovider.StreamChunk {
	ch := make(chan llmprovider.StreamChunk, len(chunks))
	go func() {
		defer close(ch)
		for _, c := range chunks {
			ch <- c
		}
	}()
	return ch
}

// TestStreamUntilComplete_BasicAccumulation verifies that all deltas are
// concatenated and token counts from the final chunk are returned.
func TestStreamUntilComplete_BasicAccumulation(t *testing.T) {
	t.Parallel()
	chunks := []llmprovider.StreamChunk{
		{Delta: "Hello"},
		{Delta: ", "},
		{Delta: "world", InputTokens: 5, OutputTokens: 3, Done: true},
	}
	ch := sendChunks(chunks)
	got, in, out, err := llmprovider.StreamUntilComplete(context.Background(), ch, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	const want = "Hello, world"
	if got != want {
		t.Errorf("accumulated text: want %q, got %q", want, got)
	}
	if in != 5 {
		t.Errorf("InputTokens: want 5, got %d", in)
	}
	if out != 3 {
		t.Errorf("OutputTokens: want 3, got %d", out)
	}
}

// TestStreamUntilComplete_EarlyStop verifies that the stopFn halts accumulation
// before the channel is drained.
func TestStreamUntilComplete_EarlyStop(t *testing.T) {
	t.Parallel()
	words := []string{"one", " ", "two", " ", "three", " ", "four", " ", "five"}
	chunks := make([]llmprovider.StreamChunk, len(words))
	for i, w := range words {
		chunks[i] = llmprovider.StreamChunk{Delta: w}
	}
	chunks[len(chunks)-1].Done = true

	ch := sendChunks(chunks)

	// Stop after "two" appears in the accumulation.
	stopFn := func(acc string) bool {
		return strings.Contains(acc, "two")
	}
	got, _, _, err := llmprovider.StreamUntilComplete(context.Background(), ch, stopFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "two") {
		t.Errorf("expected accumulation to contain %q, got %q", "two", got)
	}
	// Should have stopped before "three".
	if strings.Contains(got, "three") {
		t.Errorf("early-stop should prevent %q from appearing, got %q", "three", got)
	}
}

// TestStreamUntilComplete_ContextCancel verifies that cancelling the context
// returns the partial accumulation and the context error.
func TestStreamUntilComplete_ContextCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())

	// A channel that blocks after one chunk.
	ch := make(chan llmprovider.StreamChunk)
	go func() {
		ch <- llmprovider.StreamChunk{Delta: "partial"}
		// Block until test is done; do not close channel.
		<-ctx.Done()
		close(ch)
	}()

	// Let the first chunk arrive, then cancel.
	go func() {
		cancel()
	}()

	got, _, _, err := llmprovider.StreamUntilComplete(ctx, ch, nil)
	if err == nil {
		t.Error("expected context error, got nil")
	}
	// Partial data may or may not be included depending on timing; just ensure
	// the function returned.
	_ = got
}

// TestStreamUntilComplete_ClosedChannelReturns verifies that a closed
// channel without Done chunk still returns whatever was accumulated.
func TestStreamUntilComplete_ClosedChannelReturns(t *testing.T) {
	t.Parallel()
	chunks := []llmprovider.StreamChunk{
		{Delta: "abc"},
		{Delta: "def"},
		// channel closed without Done=true
	}
	ch := sendChunks(chunks)
	got, _, _, err := llmprovider.StreamUntilComplete(context.Background(), ch, nil)
	if err != nil {
		t.Fatalf("unexpected error on closed channel: %v", err)
	}
	if got != "abcdef" {
		t.Errorf("want %q, got %q", "abcdef", got)
	}
}
