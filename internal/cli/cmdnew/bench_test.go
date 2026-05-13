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
