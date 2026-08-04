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

// hostprovider.go — the provider identity used when the reasoning plane is
// the host agent rather than a paid API (see internal/agentbridge).
//
// HostProvider deliberately has no transport. Every completion in agent mode
// is served by the bridge's replay store before dispatch ever happens, so a
// call reaching Complete means the bridge was bypassed — a wiring bug, and it
// is reported as one instead of silently degrading to an empty completion.
package llmprovider

import (
	"context"

	"github.com/teragrid/forge/internal/errcode"
)

// HostProviderName is the provider name reported in agent mode. It shows up
// in the token ledger and in `forge ship` output, so it is worth being
// explicit that no vendor API was involved.
const HostProviderName = "host-agent"

// HostContextWindow is the input budget assumed for the host agent.
//
// This governs how much knowledge-base material forge injects into a prompt.
// It is set to a conservative 200k — the smallest context window among the
// chat surfaces this mode targets (Claude Code, Copilot Chat, Cursor) — so
// prompts stay inside the window of whichever host is actually driving,
// rather than being sized for the most generous one.
const HostContextWindow = 200_000

// HostProvider represents "a human-facing agent supplies the completion".
type HostProvider struct{}

// NewHostProvider returns the provider used in agent mode.
func NewHostProvider() *HostProvider { return &HostProvider{} }

// Name implements Provider.
func (h *HostProvider) Name() string { return HostProviderName }

// Capabilities implements Provider. The model list is a single synthetic id;
// tier routing has nothing to choose between when there is one host.
func (h *HostProvider) Capabilities() Capabilities {
	return Capabilities{
		Streaming: false,
		MaxTokens: HostContextWindow,
		Models:    []string{HostProviderName},
	}
}

// Complete implements Provider. Reaching it is always a bug: in agent mode
// the bridge resolves or defers every prompt before dispatch.
func (h *HostProvider) Complete(_ context.Context, _ *Request) (*Response, error) {
	return nil, errcode.New(ErrProviderFail,
		"host-agent provider has no transport: the agent bridge must resolve or defer "+
			"every prompt before dispatch (this is a forge wiring bug, not a configuration error)",
		nil)
}
