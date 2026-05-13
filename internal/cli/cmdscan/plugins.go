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
// Package cmdscan: in-tree plugin adapters.
//
// Each builtin scanner family (secrets, rls, prompt-injection, supply-chain)
// is exposed as a plugin.Scanner and registered with plugin.Default() at
// init() time so that:
//
//   - `forge plugin list` enumerates the scanners alongside any future
//     out-of-tree plugins;
//   - downstream tooling can iterate plugin.Default().ByKind(plugin.KindScanner)
//     instead of hard-coding family names;
//   - the WASM runtime (M2.2) can register itself the same way.
//
// The legacy Run* functions remain the source of truth — these adapters
// simply delegate. Backward compatibility for `forge scan <family>` is
// preserved.
package cmdscan

import (
	"context"

	"github.com/teragrid/forge/internal/plugin"
)

// builtinScanner adapts a Run* function into a plugin.Scanner.
type builtinScanner struct {
	manifest plugin.Manifest
	runFn    func(root string) (*ScanResult, error)
}

func (b *builtinScanner) Manifest() plugin.Manifest { return b.manifest }

func (b *builtinScanner) Scan(_ context.Context, root string) ([]plugin.Finding, error) {
	res, err := b.runFn(root)
	if err != nil {
		return nil, err
	}
	out := make([]plugin.Finding, 0, len(res.Findings))
	for _, f := range res.Findings {
		out = append(out, plugin.Finding{
			File:   f.File,
			Line:   f.Line,
			Rule:   f.Rule,
			Match:  f.Match,
			Detail: f.Secret,
		})
	}
	return out, nil
}

// builtinScanners is the static list of in-tree scanner adapters.
// Exposed as a function so tests can call it against a private registry.
func builtinScanners() []plugin.Plugin {
	return []plugin.Plugin{
		&builtinScanner{
			manifest: plugin.Manifest{
				Name: "secrets", Version: "1.0.0", Kind: plugin.KindScanner,
				Author: "forge", Summary: "Detect API keys, tokens, private keys (gitleaks fallback).",
			},
			runFn: RunSecrets,
		},
		&builtinScanner{
			manifest: plugin.Manifest{
				Name: "rls", Version: "1.0.0", Kind: plugin.KindScanner,
				Author: "forge", Summary: "Flag SQL/migration files missing tenant/workspace scoping.",
			},
			runFn: RunRLS,
		},
		&builtinScanner{
			manifest: plugin.Manifest{
				Name: "prompt-injection", Version: "1.0.0", Kind: plugin.KindScanner,
				Author: "forge", Summary: "Flag known prompt-injection patterns in templates and prose.",
			},
			runFn: RunPromptInjection,
		},
		&builtinScanner{
			manifest: plugin.Manifest{
				Name: "supply-chain", Version: "1.0.0", Kind: plugin.KindScanner,
				Author: "forge", Summary: "Flag risky pinning and curl-pipe-shell in dependency files.",
			},
			runFn: RunSupplyChain,
		},
	}
}

func init() {
	reg := plugin.Default()
	for _, p := range builtinScanners() {
		// Avoid duplicate-panic if init() is re-entered in tests that
		// import cmdscan via multiple paths.
		if _, ok := reg.Lookup(p.Manifest().Name); ok {
			continue
		}
		reg.Register(p)
	}
}
