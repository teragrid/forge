//go:build !kb_private

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

package knowledge

// encryptedIndex is nil in public / open-source builds.
// The knowledge.Load() function treats nil as a valid empty index so the
// binary compiles and operates without the private knowledge base.
//
// To build with the full encrypted knowledge index:
//  1. Obtain the private forge-knowledge/ directory.
//  2. Run: go run ./cmd/gen-knowledge-index
//     This writes internal/knowledge/zdata_private.go (gitignored).
//  3. Build: go build -tags kb_private ./...
var encryptedIndex []byte
