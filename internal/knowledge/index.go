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
	"encoding/json"
	"sync"

	"github.com/teragrid/forge/internal/errcode"
)

// ErrIndexLoad is returned when the embedded index cannot be decrypted/parsed.
var ErrIndexLoad = errcode.Register(errcode.Code(6300), "knowledge index load failed")

var (
	globalIndex *Index
	loadOnce    sync.Once
	loadErr     error
)

// Load returns the singleton Index. In private builds (go build -tags kb_private)
// the full encrypted knowledge base is loaded and decrypted. In public/stub builds
// an empty index is returned so callers degrade gracefully.
//
// It is safe for concurrent use. After the first call the result is cached.
func Load() (*Index, error) {
	loadOnce.Do(func() {
		plain, err := decryptIndex(encryptedIndex)
		if err != nil {
			loadErr = errcode.Newf(ErrIndexLoad, err, "decrypt knowledge index")
			return
		}
		var idx Index
		if err := json.Unmarshal(plain, &idx); err != nil {
			loadErr = errcode.Newf(ErrIndexLoad, err, "parse knowledge index")
			return
		}
		globalIndex = &idx
	})
	return globalIndex, loadErr
}

// MustLoad is like Load but panics on error. Use only in tests or init paths
// where the data is known-good.
func MustLoad() *Index {
	idx, err := Load()
	if err != nil {
		panic("knowledge.MustLoad: " + err.Error())
	}
	return idx
}
