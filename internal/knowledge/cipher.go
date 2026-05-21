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

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"io"
)

// knowledgeKey is the AES-128 symmetric key for the embedded knowledge index.
//
// Override at private build time with:
//
//	-ldflags "-X 'github.com/teragrid/forge/internal/knowledge.knowledgeKey=YOUR16BYTEKEY!'"
//
// The key must be exactly 16 bytes (it is zero-padded or truncated to fit).
// Changing the key requires regenerating the index with the same key passed to
// cmd/gen-knowledge-index via the --key flag.
//
//nolint:gosec // intentional compile-time constant — content is IP, key is not the secret
var knowledgeKey = "forge-kb-2024-v1"

// decryptIndex decrypts and decompresses the private encrypted knowledge index.
//
// Wire format: nonce(12 bytes) || AES-128-GCM(gzip(JSON))
//
// Returns the raw JSON bytes on success.
// When data is nil (stub / open-source build), returns a valid empty index JSON
// so the package works transparently without the private knowledge base.
func decryptIndex(data []byte) ([]byte, error) {
	if len(data) == 0 {
		// stub / public build — return a valid empty index so callers succeed.
		return []byte(`{"version":"1","entries":[]}`), nil
	}

	const nonceSize = 12
	const gcmTagSize = 16
	if len(data) < nonceSize+gcmTagSize {
		return nil, fmt.Errorf("invalid encrypted index: data too short (%d bytes)", len(data))
	}

	key := kbKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("knowledge cipher: init: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("knowledge cipher: gcm: %w", err)
	}

	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	compressed, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("knowledge cipher: decrypt: %w", err)
	}

	r, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("knowledge cipher: decompress: %w", err)
	}
	defer r.Close()

	plain, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("knowledge cipher: read: %w", err)
	}
	return plain, nil
}

// kbKey returns the AES-128 key as a 16-byte slice, zero-padded or truncated.
func kbKey() []byte {
	key := make([]byte, 16)
	copy(key, []byte(knowledgeKey))
	return key
}
