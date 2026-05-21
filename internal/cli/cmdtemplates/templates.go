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

// Package cmdtemplates implements `forge templates list`.
package cmdtemplates

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/scaffold"
	"github.com/teragrid/forge/internal/verbmeta"
)

// communityTemplates is the hard-coded community template registry.
// Phase 4 will replace this with a remote registry fetch.
var communityTemplates = []communityTemplate{
	{
		ID:          "enterprise-cloud-native",
		Description: "TSD-driven enterprise SaaS scaffold (multi-tenant, RBAC, audit-log, feature-flags).",
		Mode:        "tsd",
		Tags:        []string{"enterprise", "saas", "tsd", "community"},
	},
	{
		ID:          "go-cloud-native",
		Description: "Go + Chi + Neon + GCP cloud-native service.",
		Mode:        "tsd",
		Tags:        []string{"go", "cloud-native", "gcp"},
	},
	{
		ID:          "marketplace-platform",
		Description: "Next.js + Go + Adyen marketplace with multi-tenant payments.",
		Mode:        "tsd",
		Tags:        []string{"marketplace", "payments", "nextjs"},
	},
	{
		ID:          "data-platform",
		Description: "Python + FastAPI + dbt + Metabase data platform.",
		Mode:        "tsd",
		Tags:        []string{"data", "python", "dbt"},
	},
}

// communityTemplate describes a registered community template.
type communityTemplate struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Mode        string   `json:"mode"` // "tsd" | "classic"
	Tags        []string `json:"tags"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "templates",
		Summary: "Browse and manage Forge project templates.",
		Inputs:  []string{"list — list available templates", "--json — machine-readable output"},
		Outputs: []string{"stdout — template list"},
	})
}

// New returns the `forge templates` cobra command.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "Browse Forge project templates.",
	}
	cmd.AddCommand(newListCmd())
	return cmd
}

func newListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available community and built-in templates.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Combine classic (embedded) + community TSD templates.
			type entry struct {
				ID          string   `json:"id"`
				Description string   `json:"description"`
				Mode        string   `json:"mode"`
				Tags        []string `json:"tags"`
			}

			var all []entry
			for _, t := range scaffold.AvailableTemplates() {
				all = append(all, entry{
					ID:          t,
					Description: "Built-in scaffold template.",
					Mode:        "classic",
					Tags:        []string{"built-in"},
				})
			}
			for _, t := range communityTemplates {
				all = append(all, entry{
					ID:          t.ID,
					Description: t.Description,
					Mode:        t.Mode,
					Tags:        t.Tags,
				})
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]interface{}{"templates": all})
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Available templates:")
			for _, t := range all {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-35s [%s] %s\n", t.ID, t.Mode, t.Description)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}
