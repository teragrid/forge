// Package cmdplugin implements `forge plugin` (M2.x).
// Subcommands:
//   - list  — enumerate registered plugins (text or JSON)
//   - show  — inspect a single plugin's manifest
//
// In-process (in-tree) plugins are registered via init() in their owning
// packages (e.g. cmdscan, cmdupgrade). The wazero-backed WASM loader will
// register dynamic plugins here in M2.2.
package cmdplugin

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/plugin"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 3500..3599).
var (
	ErrPluginUnknown = errcode.Register(errcode.Code(3500), "unknown plugin")
	ErrPluginUsage   = errcode.Register(errcode.Code(3501), "plugin command usage error")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "plugin",
		Summary: "Inspect and manage Forge plugins (in-tree scanners, codemods, future WASM).",
		Inputs: []string{
			"<subcommand>: list | show",
			"--kind <scanner|codemod|provider|template> (filter, list only)",
			"--json (machine-readable output)",
		},
		Outputs:      []string{"stdout: plugin table or JSON"},
		SideEffects:  []string{"read-only"},
		GatesTouched: []string{"§16.5.4 #5 — supply-chain visibility"},
		ErrorCodes:   []errcode.Code{ErrPluginUnknown, ErrPluginUsage},
	})
}

// New returns the root cobra command for `forge plugin`.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Inspect and manage Forge plugins (M2.x).",
	}
	cmd.AddCommand(newListCmd(), newShowCmd())
	return cmd
}

func newListCmd() *cobra.Command {
	var (
		asJSON   bool
		kindFlag string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered plugins.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			plugins := plugin.Default().All()
			if kindFlag != "" {
				k := plugin.Kind(kindFlag)
				switch k {
				case plugin.KindScanner, plugin.KindCodemod, plugin.KindProvider, plugin.KindTemplate:
				default:
					return errcode.Newf(ErrPluginUsage, nil,
						"unknown --kind %q; one of: scanner, codemod, provider, template", kindFlag)
				}
				plugins = plugin.Default().ByKind(k)
			}

			if asJSON {
				out := make([]plugin.Manifest, 0, len(plugins))
				for _, p := range plugins {
					out = append(out, p.Manifest())
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tKIND\tVERSION\tSUMMARY")
			for _, p := range plugins {
				m := p.Manifest()
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.Name, m.Kind, m.Version, m.Summary)
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	cmd.Flags().StringVar(&kindFlag, "kind", "", "filter by kind (scanner|codemod|provider|template)")
	return cmd
}

func newShowCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show a plugin's manifest.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			p, ok := plugin.Default().Lookup(name)
			if !ok {
				return errcode.Newf(ErrPluginUnknown, nil, "no plugin named %q", name)
			}
			m := p.Manifest()
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(m)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "name:    %s\n", m.Name)
			fmt.Fprintf(w, "kind:    %s\n", m.Kind)
			fmt.Fprintf(w, "version: %s\n", m.Version)
			if m.Author != "" {
				fmt.Fprintf(w, "author:  %s\n", m.Author)
			}
			if m.Summary != "" {
				fmt.Fprintf(w, "summary: %s\n", m.Summary)
			}
			if m.Forge != "" {
				fmt.Fprintf(w, "forge:   %s\n", m.Forge)
			}
			if len(m.Capabilities) > 0 {
				fmt.Fprintf(w, "caps:    %s\n", strings.Join(m.Capabilities, ", "))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}
