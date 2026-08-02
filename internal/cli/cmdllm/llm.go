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

// Package cmdllm implements `forge llm` — a one-stop command for inspecting
// and switching which LLM backend forge ship/spec/arch/etc. call, so that
// doesn't require hand-editing forge.yml or knowing which environment
// variable each provider reads.
//
//	forge llm            — alias for `forge llm list`
//	forge llm list        — show every known provider, which are usable
//	                         right now given current credentials, which one
//	                         is currently active, and (for Copilot) the live
//	                         model catalog with the resolved default
//	forge llm use <provider> [model]
//	                       — switch forge.yml's llm.provider (and llm.model,
//	                         or clear it when omitted so the provider picks
//	                         its own default), then makes one real
//	                         completion call to confirm it actually works
//	                         before declaring success
//
// This exists because `forge doctor --llm` already answers "is the
// *currently configured* provider healthy", but there was no single command
// to discover what else is available or switch to it without manually
// running `forge config set llm.provider ...` + `forge config set
// llm.model ...` + `forge doctor --llm` as three separate steps and
// guessing model ids from memory.
package cmdllm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/config"
	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/llmprovider"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 2010..2019 — free block confirmed against the
// rest of internal/cli's 20xx registrations at the time this was written).
var (
	ErrLLMUnknownProvider = errcode.Register(errcode.Code(2010), "unknown LLM provider name")
	ErrLLMNoCredentials   = errcode.Register(errcode.Code(2011), "LLM provider credentials unavailable")
	ErrLLMModelUnknown    = errcode.Register(errcode.Code(2012), "LLM model not in provider's live catalog")
	ErrLLMCheckFailed     = errcode.Register(errcode.Code(2013), "post-switch LLM live check failed")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "llm",
		Summary: "Inspect and switch which LLM backend forge ship/spec/etc. use.",
		Inputs: []string{
			"list                   — show providers, availability, current selection, live Copilot models",
			"use <provider> [model] — switch forge.yml llm.provider (+ llm.model), then live-verify",
			"--root <path>          — project root (default: cwd)",
			"--json                 — machine-readable output",
			"--skip-check           — (use only) write the config without a live verification call",
		},
		Outputs:      []string{"stdout: provider/model table (text) or JSON object"},
		SideEffects:  []string{"`use`: writes forge.yml llm.provider/llm.model; `list`: none (read-only)"},
		GatesTouched: []string{"§16.5.4 — config correctness"},
		ErrorCodes:   []errcode.Code{ErrLLMUnknownProvider, ErrLLMNoCredentials, ErrLLMModelUnknown, ErrLLMCheckFailed},
		OutputFields: []string{"providers", "active"},
	})
}

// New returns the `forge llm` cobra command.
func New() *cobra.Command {
	var (
		root   string
		asJSON bool
	)

	cmd := &cobra.Command{
		Use:   "llm",
		Short: "Inspect and switch which LLM backend forge uses.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runList(cmd, root, asJSON)
		},
	}
	cmd.PersistentFlags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.PersistentFlags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Show known providers, availability, and the active selection.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runList(cmd, root, asJSON)
		},
	}

	var skipCheck bool
	useCmd := &cobra.Command{
		Use:   "use <provider> [model]",
		Short: "Switch the active LLM provider (and optionally model), then verify it works.",
		Long: "Writes forge.yml's llm.provider (and llm.model, if given) and makes one real,\n" +
			"minimal completion call to confirm the new configuration actually works —\n" +
			"catching a bad model id or missing credentials immediately instead of on the\n" +
			"next `forge ship`.\n\n" +
			"Examples:\n" +
			"  forge llm use anthropic\n" +
			"  forge llm use copilot claude-sonnet-5\n" +
			"  forge llm use copilot          (clears any pinned model — resolves the\n" +
			"                                  best available model from the live\n" +
			"                                  Copilot catalog automatically)\n\n" +
			"Valid providers: " + strings.Join(llmprovider.KnownProviderNames(), ", "),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := args[0]
			model := ""
			if len(args) == 2 {
				model = args[1]
			}
			return runUse(cmd, root, provider, model, asJSON, skipCheck)
		},
	}
	useCmd.Flags().BoolVar(&skipCheck, "skip-check", false, "write the config without making a live verification call")

	cmd.AddCommand(listCmd, useCmd)
	return cmd
}

// ── list ──────────────────────────────────────────────────────────────────

type providerRow struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Active    bool   `json:"active"`
}

type copilotModelRow struct {
	ID            string `json:"id"`
	Vendor        string `json:"vendor"`
	Enabled       bool   `json:"enabled"`
	PickerEnabled bool   `json:"model_picker_enabled"`
	IsResolved    bool   `json:"is_resolved_default"`
}

func runList(cmd *cobra.Command, root string, asJSON bool) error {
	cfg := loadCfg(root)

	// "Active" here means whichever provider Detect() would actually pick
	// right now — the same resolution `forge ship` itself uses (forge.yml
	// llm.provider if set to something other than "auto", else env-var
	// auto-detection order) — not just what forge.yml literally says, since
	// an unset/"auto" provider is resolved dynamically.
	var activeName string
	if active, err := llmprovider.Detect(); err == nil {
		activeName = active.Name()
	}

	rows := make([]providerRow, 0, len(llmprovider.KnownProviderNames()))
	var copilotProvider *llmprovider.CopilotProvider
	for _, name := range llmprovider.KnownProviderNames() {
		p := llmprovider.DetectByName(name)
		row := providerRow{Name: name, Available: p != nil}
		if p != nil {
			row.Active = matchesActive(p.Name(), name, activeName)
			if cp, ok := p.(*llmprovider.CopilotProvider); ok {
				copilotProvider = cp
			}
		}
		rows = append(rows, row)
	}

	var copilotModels []copilotModelRow
	if copilotProvider != nil {
		infos, live := copilotProvider.LiveModels()
		resolved := copilotProvider.ResolvedModel()
		copilotModels = make([]copilotModelRow, 0, len(infos))
		for _, m := range infos {
			copilotModels = append(copilotModels, copilotModelRow{
				ID: m.ID, Vendor: m.Vendor, Enabled: m.Enabled, PickerEnabled: m.PickerEnabled,
				IsResolved: m.ID == resolved,
			})
		}
		sort.Slice(copilotModels, func(i, j int) bool {
			if copilotModels[i].IsResolved != copilotModels[j].IsResolved {
				return copilotModels[i].IsResolved // resolved default first
			}
			return copilotModels[i].ID < copilotModels[j].ID
		})
		_ = live // surfaced in text output below; not load-bearing for JSON shape
	}

	if asJSON {
		out := map[string]any{
			"providers":           rows,
			"configured_provider": cfg.LLMProvider.Raw,
			"configured_model":    cfg.LLMModel.Raw,
			"active":              activeName,
		}
		if copilotProvider != nil {
			out["copilot_models"] = copilotModels
		}
		return writeJSON(cmd, out)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "forge LLM providers (file: %s)\n\n", cfg.FilePath)
	fmt.Fprintf(w, "%-18s  %-11s  %s\n", "PROVIDER", "AVAILABLE", "")
	fmt.Fprintf(w, "%-18s  %-11s  %s\n", "--------", "---------", "")
	for _, r := range rows {
		mark := ""
		if r.Active {
			mark = "← active"
		}
		avail := "no"
		if r.Available {
			avail = "yes"
		}
		fmt.Fprintf(w, "%-18s  %-11s  %s\n", r.Name, avail, mark)
	}
	fmt.Fprintf(w, "\nconfigured: llm.provider=%q llm.model=%q (%q means auto-detect)\n",
		valueOrUnset(cfg.LLMProvider.Raw), valueOrUnset(cfg.LLMModel.Raw), "auto")

	if copilotProvider != nil {
		_, live := copilotProvider.LiveModels()
		src := "live GET /models"
		if !live {
			src = "static fallback list — /models was unreachable"
		}
		fmt.Fprintf(w, "\ngithub-copilot models (%s):\n", src)
		fmt.Fprintf(w, "%-28s  %-16s  %-9s  %-14s  %s\n", "MODEL", "VENDOR", "ENABLED", "PICKER", "")
		for _, m := range copilotModels {
			mark := ""
			if m.IsResolved {
				mark = "← resolved default"
			}
			fmt.Fprintf(w, "%-28s  %-16s  %-9v  %-14v  %s\n", m.ID, m.Vendor, m.Enabled, m.PickerEnabled, mark)
		}
		fmt.Fprintln(w, "\n(only models with ENABLED=true and PICKER=true are eligible to be")
		fmt.Fprintln(w, " picked automatically — see: forge llm use copilot <model> to pin one)")
	}
	fmt.Fprintln(w, "\nswitch with: forge llm use <provider> [model]")
	return nil
}

// matchesActive reports whether provider p (registered under knownName) is
// the one Detect() actually resolved to (activeName, from Provider.Name()).
// The two use different casing/formats ("copilot" vs "github-copilot",
// "anthropic" vs "anthropic") so this maps known aliases rather than a
// plain string compare.
func matchesActive(pName, knownName, activeName string) bool {
	if activeName == "" {
		return false
	}
	if pName == activeName {
		return true
	}
	aliases := map[string]string{"copilot": "github-copilot"}
	return aliases[knownName] == activeName
}

// ── use ───────────────────────────────────────────────────────────────────

func runUse(cmd *cobra.Command, root, provider, model string, asJSON, skipCheck bool) error {
	normalized := strings.ToLower(strings.TrimSpace(provider))
	valid := false
	for _, n := range llmprovider.KnownProviderNames() {
		if n == normalized {
			valid = true
			break
		}
	}
	if !valid {
		return errcode.Newf(ErrLLMUnknownProvider, nil,
			"unknown provider %q; valid: %s", provider, strings.Join(llmprovider.KnownProviderNames(), ", "))
	}

	p := llmprovider.DetectByName(normalized)
	if p == nil {
		return errcode.Newf(ErrLLMNoCredentials, nil,
			"no credentials found for provider %q — see forge.yml/README for the environment "+
				"variable or config file each provider needs (run 'forge llm list' to see all)", normalized)
	}

	// For Copilot specifically, validate an explicitly-given model against
	// the live catalog before writing anything — the API itself won't
	// reject an unknown id (it silently substitutes a different model, see
	// copilotDefaultModel's doc comment in llmprovider/copilot.go), so this
	// is the only place that actually catches a typo'd/stale model id.
	if normalized == "copilot" && model != "" {
		if cp, ok := p.(*llmprovider.CopilotProvider); ok {
			infos, _ := cp.LiveModels()
			found := false
			for _, m := range infos {
				if m.ID == model {
					found = true
					if !m.Enabled || !m.PickerEnabled {
						return errcode.Newf(ErrLLMModelUnknown, nil,
							"model %q exists but is not enabled for this account/org (policy or picker "+
								"disabled) — run 'forge llm list' to see which models are usable", model)
					}
					break
				}
			}
			if !found {
				ids := make([]string, 0, len(infos))
				for _, m := range infos {
					ids = append(ids, m.ID)
				}
				return errcode.Newf(ErrLLMModelUnknown, nil,
					"model %q not found in the live Copilot catalog; available: %s", model, strings.Join(ids, ", "))
			}
		}
	}

	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			root = "."
		}
	}
	if err := config.WriteKey(root, "llm.provider", normalized); err != nil {
		return errcode.Newf(ErrLLMNoCredentials, err, "write llm.provider")
	}
	// Always write llm.model — an explicit value when given, or "" to clear
	// any model left over from a previous provider (e.g. a GPT id that
	// means nothing to Anthropic) so the new provider resolves its own
	// default instead of failing on a stale, incompatible pin.
	if err := config.WriteKey(root, "llm.model", model); err != nil {
		return errcode.Newf(ErrLLMNoCredentials, err, "write llm.model")
	}

	result := map[string]any{
		"provider": normalized,
		"model":    model,
		"checked":  false,
	}

	if !skipCheck {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		// Re-detect from scratch so this exercises exactly what `forge ship`
		// will see next (forge.yml as just written), not the in-memory p
		// from before the write.
		fresh, err := llmprovider.Detect()
		if err != nil {
			return errcode.Newf(ErrLLMCheckFailed, err, "post-switch check: provider no longer detectable")
		}
		resp, err := fresh.Complete(ctx, &llmprovider.Request{
			UserPrompt: "Reply with exactly: ok",
			MaxTokens:  8,
			Capability: "llm-use-check",
		})
		if err != nil {
			return errcode.Newf(ErrLLMCheckFailed, err,
				"config written, but the live verification call failed — provider=%s", fresh.Name())
		}
		result["checked"] = true
		result["resolved_model"] = resp.Model
	}

	if asJSON {
		return writeJSON(cmd, result)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "llm.provider = %s\n", normalized)
	fmt.Fprintf(w, "llm.model    = %s\n", valueOrUnset(model))
	if !skipCheck {
		fmt.Fprintf(w, "\nlive check: OK — resolved model = %s\n", result["resolved_model"])
	} else {
		fmt.Fprintln(w, "\n(skipped live check — run 'forge doctor --llm' to verify)")
	}
	return nil
}

// ── shared helpers ────────────────────────────────────────────────────────

func loadCfg(root string) *config.Config {
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			root = "."
		}
	}
	cfg, err := config.Load(root, nil)
	if err != nil {
		d, _ := config.Load(root, nil)
		return d
	}
	return cfg
}

func valueOrUnset(v string) string {
	if v == "" {
		return "(unset)"
	}
	return v
}

func writeJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
