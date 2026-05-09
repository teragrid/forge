// Package main is the entry point for the `forge` CLI binary.
//
// The CLI is a thin shell over verbs registered in internal/cli. Per ADR-001,
// the command tree uses cobra and viper for arg/config plumbing. Per ADR-002,
// any plugin invocation goes through the wazero host and is gated by build
// tags (`-tags forge_wasmtime` for the cgo-backed escape hatch).
package main

import (
	"os"

	"github.com/teragrid/forge/internal/cli"
)

// Version is overridden at build time via -ldflags "-X main.Version=...".
var Version = "0.0.0-dev"

func main() {
	if err := cli.NewRootCommand(Version).Execute(); err != nil {
		os.Exit(1)
	}
}
