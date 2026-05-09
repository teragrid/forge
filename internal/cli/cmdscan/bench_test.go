package cmdscan

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkScanSecrets_500Files measures builtin secrets scanner on a 500-file
// fixture (NFR §16.4: scan ≤2s for 1k files; this is the half-size sentinel).
func BenchmarkScanSecrets_500Files(b *testing.B) {
	root := b.TempDir()
	for i := 0; i < 500; i++ {
		dir := filepath.Join(root, fmt.Sprintf("d%03d", i/50))
		_ = os.MkdirAll(dir, 0o755)
		body := []byte("package x\nfunc Foo() {}\n// nothing secret here\n")
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%03d.go", i)), body, 0o600); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = scanWithBuiltinPatterns(root)
	}
}
