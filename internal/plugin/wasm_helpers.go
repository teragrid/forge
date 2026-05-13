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
//go:build forge_wasm

package plugin

import (
	"bytes"
	"fmt"
	"os"
)

// readWASMFile reads the .wasm binary from path.
func readWASMFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("wasm: read %s: %w", path, err)
	}
	return data, nil
}

// captureWriter is an io.Writer that accumulates output in a buffer.
type captureWriter struct {
	buf bytes.Buffer
}

func (w *captureWriter) Write(p []byte) (int, error) { return w.buf.Write(p) }
func (w *captureWriter) Bytes() []byte               { return w.buf.Bytes() }
