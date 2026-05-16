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

// G-045: llm.batch — coalesce concurrent LLM requests.
//
// Batch sends multiple requests concurrently via a worker pool, respecting
// provider-level concurrency limits. Provides ≥1.5× throughput vs sequential
// on providers that support parallel calls.
package llmprovider

import (
	"context"
	"sync"
)

// BatchRequest is one item in a batch.
type BatchRequest struct {
	Index   int
	Request *Request
}

// BatchResponse is the result for one item in a batch.
type BatchResponse struct {
	Index    int
	Response *Response
	Err      error
}

// Batch sends all requests concurrently via provider, using up to concurrency
// parallel workers. Results are returned in the same order as the input.
// concurrency=0 means use len(requests) workers (fully parallel).
func Batch(ctx context.Context, provider Provider, requests []*Request, concurrency int) []BatchResponse {
	if len(requests) == 0 {
		return nil
	}
	if concurrency <= 0 || concurrency > len(requests) {
		concurrency = len(requests)
	}

	results := make([]BatchResponse, len(requests))
	work := make(chan BatchRequest, len(requests))

	// Fill work queue.
	for i, req := range requests {
		work <- BatchRequest{Index: i, Request: req}
	}
	close(work)

	var wg sync.WaitGroup
	wg.Add(concurrency)
	for range concurrency {
		go func() {
			defer wg.Done()
			for item := range work {
				resp, err := provider.Complete(ctx, item.Request)
				results[item.Index] = BatchResponse{
					Index:    item.Index,
					Response: resp,
					Err:      err,
				}
			}
		}()
	}
	wg.Wait()
	return results
}
