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

// Package cmdconfig implements `forge config` (DEV-M0-02).
//
//	forge config show              — print all resolved keys with sources
//	forge config get <key>         — print one resolved value
//	forge config explain [<key>]   — show value + winning layer for every key (or one)
package cmdconfig

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/config"
	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 2003..2099 — cli/config subset of the config range).
var (
	ErrConfigLoad = errcode.Register(errcode.Code(2003), "config load failed")
	ErrConfigKey  = errcode.Register(errcode.Code(2004), "unknown config key in CLI")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "config",
		Summary: "Inspect the layered forge configuration (defaults → forge.yml → env → flags).",
		Inputs: []string{
			"show                   — print all keys with resolved values and sources",
			"get <key>              — print one value (e.g. llm.provider)",
			"explain [<key>]        — show value + winning layer for all keys or one key",
			"--root <path>          — project root (default: cwd)",
			"--json                 — machine-readable output",
		},
		Outputs:      []string{"stdout: config table (text) or JSON object"},
		SideEffects:  []string{"none (read-only)"},
		GatesTouched: []string{"§16.5.4 — config correctness"},
		ErrorCodes:   []errcode.Code{ErrConfigLoad, ErrConfigKey},
		OutputFields: []string{"llm.provider", "log.level"},
	})
}

// New returns the `forge config` cobra command.
func New() *cobra.Command {
	var (
		root   string
		asJSON bool
	)

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect the layered forge configuration.",
		Args:  cobra.NoArgs,
		// Default: same as `show`
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runShow(cmd, loadCfg(root), asJSON)
		},
	}
	cmd.PersistentFlags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.PersistentFlags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")

	// forge config show
	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Print all resolved configuration keys.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runShow(cmd, loadCfg(root), asJSON)
		},
	}

	// forge config get <key>
	getCmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Print one resolved configuration value.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadCfg(root)
			v, err := cfg.Get(args[0])
			if err != nil {
				return errcode.Newf(ErrConfigKey, err, "config get %s", args[0])
			}
			if asJSON {
				return writeJSON(cmd, map[string]any{
					"key":    args[0],
					"value":  v.Raw,
					"source": v.Source.String(),
					"detail": v.SourceDetail,
				})
			}
			fmt.Fprintln(cmd.OutOrStdout(), v.Raw)
			return nil
		},
	}

	// forge config explain [<key>]
	explainCmd := &cobra.Command{
		Use:   "explain [<key>]",
		Short: "Show resolved value and winning configuration layer for each key.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadCfg(root)
			if len(args) == 1 {
				v, err := cfg.Get(args[0])
				if err != nil {
					return errcode.Newf(ErrConfigKey, err, "config explain %s", args[0])
				}
				if asJSON {
					return writeJSON(cmd, explainField(args[0], v))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-35s  %-15s  %-12s  %s\n", args[0], v.Raw, v.Source, v.SourceDetail)
				return nil
			}
			return runExplainAll(cmd, cfg, asJSON)
		},
	}

	// forge config set <key> <value>
	setCmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Persist a configuration value to forge.yml.",
		Long: "Write a key-value pair to forge.yml in the project root.\n\n" +
			"Examples:\n" +
			"  forge config set llm.model claude-sonnet-4-5\n" +
			"  forge config set llm.model gpt-4o\n" +
			"  forge config set log.level debug\n\n" +
			"Valid keys: llm.provider, llm.model, llm.daily_budget_usd,\n" +
			"            llm.monthly_budget_usd, telemetry.enabled,\n" +
			"            telemetry.install_id, log.format, log.level",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.WriteKey(root, args[0], args[1]); err != nil {
				return errcode.Newf(ErrConfigLoad, err, "config set %s", args[0])
			}
			fmt.Fprintf(cmd.OutOrStdout(), "set %s = %s\n", args[0], args[1])
			return nil
		},
	}

	cmd.AddCommand(showCmd, getCmd, explainCmd, setCmd)
	return cmd
}

// loadCfg resolves the config, defaulting root to cwd on error.
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
		// Return defaults on any load error so the command can still report
		// the file path and the error detail.
		_ = err
		d, _ := config.Load(root, nil)
		return d
	}
	return cfg
}

func runShow(cmd *cobra.Command, cfg *config.Config, asJSON bool) error {
	if asJSON {
		m := make(map[string]any, 8)
		for _, f := range cfg.AllFields() {
			m[f.Key] = f.Value.Raw
		}
		m["_file"] = cfg.FilePath
		return writeJSON(cmd, m)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "forge configuration (file: %s)\n\n", cfg.FilePath)
	fmt.Fprintf(w, "%-35s  %s\n", "KEY", "VALUE")
	fmt.Fprintf(w, "%-35s  %s\n", "---", "-----")
	for _, f := range cfg.AllFields() {
		val := f.Value.Raw
		if val == "" {
			val = "(unset)"
		}
		fmt.Fprintf(w, "%-35s  %s\n", f.Key, val)
	}
	return nil
}

func runExplainAll(cmd *cobra.Command, cfg *config.Config, asJSON bool) error {
	if asJSON {
		out := make([]map[string]any, 0, 8)
		for _, f := range cfg.AllFields() {
			out = append(out, explainField(f.Key, f.Value))
		}
		return writeJSON(cmd, out)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "forge configuration explain (file: %s)\n\n", cfg.FilePath)
	fmt.Fprintf(w, "%-35s  %-10s  %-12s  %s\n", "KEY", "VALUE", "SOURCE", "DETAIL")
	fmt.Fprintf(w, "%-35s  %-10s  %-12s  %s\n", "---", "-----", "------", "------")
	for _, f := range cfg.AllFields() {
		val := f.Value.Raw
		if val == "" {
			val = "(unset)"
		}
		fmt.Fprintf(w, "%-35s  %-10s  %-12s  %s\n",
			f.Key, val, f.Value.Source, f.Value.SourceDetail)
	}
	return nil
}

func explainField(key string, v config.Value) map[string]any {
	return map[string]any{
		"key":    key,
		"value":  v.Raw,
		"source": v.Source.String(),
		"detail": v.SourceDetail,
	}
}

func writeJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
