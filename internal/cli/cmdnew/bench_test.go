package cmdnew

import (
	"bytes"
	"path/filepath"
	"testing"
)

// BenchmarkScaffold_GoService measures `forge new` scaffold of a go-service.
// NFR §16.4: scaffold ≤1s/op on a warm cache.
func BenchmarkScaffold_GoService(b *testing.B) {
	for i := 0; i < b.N; i++ {
		dir := b.TempDir()
		cmd := New("0.0.0-bench")
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"go-service", filepath.Join(dir, "svc"), "--name", "svc", "--module", "example.com/svc"})
		if err := cmd.Execute(); err != nil {
			b.Fatal(err)
		}
	}
}
