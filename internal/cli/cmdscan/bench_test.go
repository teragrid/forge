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
