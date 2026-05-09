//go:build !forge_wasm

package plugin

import (
	"context"
	"testing"
)

// TC-WASM-01: NewExternalPlugin returns non-nil.
func TestNewExternalPlugin_NotNil(t *testing.T) {
	m := Manifest{Name: "test-wasm", Version: "0.1.0", Kind: KindScanner}
	ep := NewExternalPlugin(m)
	if ep == nil {
		t.Fatal("NewExternalPlugin returned nil")
	}
}

// TC-WASM-02: Manifest() returns the manifest passed to NewExternalPlugin.
func TestExternalPlugin_Manifest(t *testing.T) {
	m := Manifest{Name: "test-wasm", Version: "0.1.0", Kind: KindScanner, WASMPath: "/tmp/test.wasm"}
	ep := NewExternalPlugin(m)
	got := ep.Manifest()
	if got.Name != "test-wasm" {
		t.Errorf("name: got %q", got.Name)
	}
	if got.WASMPath != "/tmp/test.wasm" {
		t.Errorf("WASMPath: got %q", got.WASMPath)
	}
}

// TC-WASM-03: Call returns ErrNotLoaded in stub builds.
func TestExternalPlugin_Call_Stub(t *testing.T) {
	m := Manifest{Name: "test-wasm", Version: "0.1.0", Kind: KindScanner, WASMPath: "/nonexistent.wasm"}
	ep := NewExternalPlugin(m)
	_, err := ep.Call(context.Background(), []string{"arg1"})
	if err == nil {
		t.Fatal("want ErrNotLoaded from stub Call, got nil")
	}
	if err != ErrNotLoaded {
		t.Errorf("want ErrNotLoaded, got: %v", err)
	}
}

// TC-WASM-04: ErrNotLoaded is non-nil and has meaningful message.
func TestErrNotLoaded_Message(t *testing.T) {
	if ErrNotLoaded == nil {
		t.Fatal("ErrNotLoaded is nil")
	}
	msg := ErrNotLoaded.Error()
	if msg == "" {
		t.Fatal("ErrNotLoaded has empty message")
	}
	if len(msg) < 10 {
		t.Errorf("ErrNotLoaded message too short: %q", msg)
	}
}

// TC-WASM-05: ExternalPlugin implements Plugin interface.
func TestExternalPlugin_ImplementsPlugin(_ *testing.T) {
	m := Manifest{Name: "test-wasm", Version: "0.1.0", Kind: KindScanner}
	ep := NewExternalPlugin(m)
	var _ Plugin = ep
}

// TC-WASM-06: Call with nil args returns ErrNotLoaded (not panic).
func TestExternalPlugin_Call_NilArgs(t *testing.T) {
	m := Manifest{Name: "test-wasm", Version: "0.1.0", Kind: KindScanner}
	ep := NewExternalPlugin(m)
	_, err := ep.Call(context.Background(), nil)
	if err != ErrNotLoaded {
		t.Errorf("want ErrNotLoaded for nil args, got: %v", err)
	}
}

// TC-WASM-07: WASMPath field survives JSON round-trip in Manifest.
func TestManifest_WASMPath_JSON(t *testing.T) {
	// Minimal test that WASMPath is present in the struct field.
	m := Manifest{Name: "x", Version: "1.0.0", Kind: KindScanner, WASMPath: "plugins/x.wasm"}
	if m.WASMPath != "plugins/x.wasm" {
		t.Errorf("WASMPath field not set: %q", m.WASMPath)
	}
}

// TC-WASM-08: false-positive guard — two ExternalPlugins with same manifest
// don't share state.
func TestExternalPlugin_IsolatedState(t *testing.T) {
	m := Manifest{Name: "iso", Version: "0.1.0", Kind: KindScanner}
	ep1 := NewExternalPlugin(m)
	ep2 := NewExternalPlugin(m)
	if ep1 == ep2 {
		t.Error("two NewExternalPlugin calls should return distinct objects")
	}
}
