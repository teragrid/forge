// Package cmdversion implements `forge version`.
package cmdversion

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/verbmeta"
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:        "version",
		Summary:     "Print the forge CLI version + build info.",
		Inputs:      []string{},
		Outputs:     []string{"stdout: version line or JSON"},
		SideEffects: []string{},
	})
}

// New returns the cobra command. The version string is injected from main so
// `-ldflags -X main.Version=...` flows through end-to-end.
func New(version string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the forge CLI version.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := struct {
				Version   string `json:"version"`
				GoVersion string `json:"go_version"`
				OS        string `json:"os"`
				Arch      string `json:"arch"`
			}{version, runtime.Version(), runtime.GOOS, runtime.GOARCH}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"forge %s (%s %s/%s)\n", info.Version, info.GoVersion, info.OS, info.Arch)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}
