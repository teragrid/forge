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
