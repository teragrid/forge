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
