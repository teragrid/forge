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

package cmdtest

import "testing"

// BenchmarkRun_Unit measures the overhead of the dry-run planner for a single family.
func BenchmarkRun_Unit(b *testing.B) {
	opts := RunOptions{Root: b.TempDir(), DryRun: true}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Run([]Family{FamilyUnit}, opts)
	}
}

// BenchmarkRun_All measures the overhead of planning all families.
func BenchmarkRun_All(b *testing.B) {
	opts := RunOptions{Root: b.TempDir(), DryRun: true}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Run(orderedFamilies, opts)
	}
}
