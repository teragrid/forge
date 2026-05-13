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

// Package plugin — wazero-backed WASM runtime (forge_wasm build tag).
// Build with: go build -tags forge_wasm ./...
// Requires:   go get github.com/tetratelabs/wazero@latest
//
// ExternalPlugin struct and ErrNotLoaded are defined in discovery.go.
// This file adds NewExternalPlugin constructor and the real Call method.
package plugin

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// NewExternalPlugin creates an ExternalPlugin backed by the .wasm binary
// referenced in m.WASMPath.
func NewExternalPlugin(m Manifest) *ExternalPlugin {
	return &ExternalPlugin{manifest: m}
}

// Call instantiates the WASM module, passes args as CLI arguments, captures
// stdout, and returns the raw output bytes.
func (e *ExternalPlugin) Call(ctx context.Context, args []string) ([]byte, error) {
	if e.manifest.WASMPath == "" {
		return nil, fmt.Errorf("plugin/wasm %q: WASMPath is empty", e.manifest.Name)
	}

	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	wasmBytes, err := readWASMFile(e.manifest.WASMPath)
	if err != nil {
		return nil, fmt.Errorf("plugin/wasm %q: read %s: %w", e.manifest.Name, e.manifest.WASMPath, err)
	}

	var stdout captureWriter
	cfg := wazero.NewModuleConfig().
		WithStdout(&stdout).
		WithArgs(append([]string{e.manifest.Name}, args...)...)

	_, err = rt.InstantiateWithConfig(ctx, wasmBytes, cfg)
	if err != nil {
		return nil, fmt.Errorf("plugin/wasm %q: instantiate: %w", e.manifest.Name, err)
	}
	return stdout.Bytes(), nil
}
