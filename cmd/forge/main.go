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
// Package main is the entry point for the `forge` CLI binary.
//
// The CLI is a thin shell over verbs registered in internal/cli. Per ADR-001,
// the command tree uses cobra and viper for arg/config plumbing. Per ADR-002,
// any plugin invocation goes through the wazero host and is gated by build
// tags (`-tags forge_wasmtime` for the cgo-backed escape hatch).
package main

import (
	"os"
	"runtime/debug"
	"strings"

	"github.com/teragrid/forge/internal/cli"
)

// Version is overridden at build time via -ldflags "-X main.Version=...".
// When installed via "go install", debug.ReadBuildInfo returns the module version.
var Version = "0.0.0-dev"

func init() {
	if Version != "0.0.0-dev" {
		return // already set via -ldflags at build time
	}
	if info, ok := debug.ReadBuildInfo(); ok &&
		info.Main.Version != "" &&
		info.Main.Version != "(devel)" {
		Version = strings.TrimPrefix(info.Main.Version, "v")
	}
}

func main() {
	if err := cli.NewRootCommand(Version).Execute(); err != nil {
		os.Exit(1)
	}
}
