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
//go:build !forge_wasm

// Package plugin — WASM stub for non-forge_wasm builds.
// When compiled WITHOUT the forge_wasm build tag, WASM execution is not
// available and Call() returns ErrNotLoaded.
//
// ExternalPlugin and ErrNotLoaded are defined in discovery.go;
// this file only adds the Call method and NewExternalPlugin constructor.
package plugin

import "context"

// NewExternalPlugin returns an ExternalPlugin handle. The WASM binary is
// NOT loaded in stub builds; Call will return ErrNotLoaded.
func NewExternalPlugin(m Manifest) *ExternalPlugin {
	return &ExternalPlugin{manifest: m}
}

// Call is a no-op in stub builds. It always returns ErrNotLoaded.
func (e *ExternalPlugin) Call(_ context.Context, _ []string) ([]byte, error) {
	return nil, ErrNotLoaded
}
