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

package cli

import (
	"os"
	"testing"
)

// TestMain sets FORGE_NO_LLM=1 for all tests in this package so that
// end-to-end tests using cmd.Execute() do not attempt real network calls
// to the Copilot / LLM API.
func TestMain(m *testing.M) {
	if os.Getenv("FORGE_NO_LLM") == "" {
		os.Setenv("FORGE_NO_LLM", "1") // process exits via os.Exit below; no defer needed
	}
	os.Exit(m.Run())
}
