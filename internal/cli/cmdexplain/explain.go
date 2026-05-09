// Package cmdexplain implements `forge explain` (DEV-M0-12). With no arg it
// lists every registered verb; with one arg it prints that verb's manifest.
package cmdexplain

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 1400..1499).
var (
	ErrUnknownVerb = errcode.Register(errcode.Code(1400), "unknown verb passed to `forge explain`")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "explain",
		Summary: "Print the manifest for a verb (inputs, outputs, side-effects, error codes).",
		Inputs: []string{
			"<verb> (optional; if absent, list all verbs)",
			"--json (machine-readable output)",
		},
		Outputs:     []string{"stdout: per-verb manifest (text or JSON)"},
		SideEffects: []string{},
	})
}

// New returns the cobra command.
func New() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "explain [verb]",
		Short: "Show what a verb does (inputs, outputs, side-effects).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				all := verbmeta.All()
				if asJSON {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					return enc.Encode(all)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Verbs (%d):\n", len(all))
				for _, m := range all {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-10s %s\n", m.Verb, m.Summary)
				}
				return nil
			}
			m, ok := verbmeta.Lookup(args[0])
			if !ok {
				known := []string{}
				for _, v := range verbmeta.All() {
					known = append(known, v.Verb)
				}
				return errcode.Newf(ErrUnknownVerb, nil,
					"unknown verb %q; known: %s", args[0], strings.Join(known, ", "))
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(m)
			}
			renderText(cmd, m)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

func renderText(cmd *cobra.Command, m verbmeta.Manifest) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "verb:    %s\n", m.Verb)
	fmt.Fprintf(w, "summary: %s\n", m.Summary)
	bullets(w, "inputs", m.Inputs)
	bullets(w, "outputs", m.Outputs)
	bullets(w, "side-effects", m.SideEffects)
	if len(m.GatesTouched) > 0 {
		bullets(w, "gates touched", m.GatesTouched)
	}
	if len(m.ErrorCodes) > 0 {
		strs := make([]string, len(m.ErrorCodes))
		for i, c := range m.ErrorCodes {
			strs[i] = fmt.Sprintf("%s — %s", c, errcode.Description(c))
		}
		bullets(w, "error codes", strs)
	}
}

func bullets(w interface{ Write([]byte) (int, error) }, label string, items []string) {
	fmt.Fprintf(w.(interface{ Write([]byte) (int, error) }), "%s:\n", label)
	if len(items) == 0 {
		fmt.Fprintf(w.(interface{ Write([]byte) (int, error) }), "  (none)\n")
		return
	}
	for _, it := range items {
		fmt.Fprintf(w.(interface{ Write([]byte) (int, error) }), "  - %s\n", it)
	}
}
