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

// Package cmdtemplates implements `forge templates list` and `forge templates init --from <id>`.
package cmdtemplates

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/scaffold"
	"github.com/teragrid/forge/internal/verbmeta"
)

// ErrUnknownTemplate is returned when the requested template ID is not registered.
var ErrUnknownTemplate = errcode.Register(errcode.Code(6460), "unknown template ID")

// ErrTSDExists is returned when .forge/tsd.yml already exists and --overwrite is not set.
var ErrTSDExists = errcode.Register(errcode.Code(6461), ".forge/tsd.yml already exists")

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
	{
		ID:          "promotiai",
		Description: "PromotAI — AI-native SaaS with Next.js 15, FastAPI, LangGraph, Stripe + PayPal, Celery, multi-tenant RBAC.",
		Mode:        "tsd",
		Tags:        []string{"saas", "ai", "payments", "python", "nextjs"},
	},
}

// tsdTemplateContent holds the embedded TSD YAML for each registered TSD template.
// The map key matches communityTemplate.ID.
var tsdTemplateContent = map[string]string{
	"promotiai": `tsd_version: 1
project:
  name: "{{project_name}}"
  domain: "{{domain}}"
  type: saas

stack:
  frontend:
    framework: nextjs-15
    ui_library: shadcn
    state_management: server-components-only
    testing: jest-rtl

  backend:
    language: python
    framework: fastapi
    api_style: rest
    auth: supabase-auth
    testing: pytest

  database:
    primary: supabase-pg
    cache: redis
    search: pg-fts
    migrations: supabase-migrations

  ai:
    orchestration: langgraph
    llm_providers:
      - openai
      - anthropic
      - google-gemini
    embedding: openai-ada
    vector_store: supabase-pgvector
    observability: langsmith

  payments:
    providers:
      - stripe
      - paypal
    model: subscription

  messaging:
    queue: celery-redis
    realtime: supabase-realtime
    email: resend
    sms: none

  infra:
    cloud: digitalocean
    container: docker-compose
    cdn: cloudflare
    secrets: doppler
    ci_cd: github-actions

  observability:
    metrics: prometheus-grafana
    tracing: opentelemetry
    logging: structlog
    alerting: slack-webhook

  compliance:
    standards:
      - pci-dss-saq-a
      - gdpr
    secret_scanning: gitleaks
`,
	"go-cloud-native": `tsd_version: 1
project:
  name: "{{project_name}}"
  domain: "{{domain}}"
  type: api-product

stack:
  frontend:
    framework: none

  backend:
    language: go
    framework: chi
    api_style: rest
    auth: custom-jwt
    testing: go-test

  database:
    primary: supabase-pg
    cache: none
    migrations: supabase-migrations

  infra:
    cloud: gcp
    container: kubernetes
    ci_cd: github-actions

  observability:
    metrics: prometheus-grafana
    tracing: opentelemetry
    logging: structlog
`,
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
	cmd.AddCommand(newInitCmd())
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

func newInitCmd() *cobra.Command {
	var (
		fromID    string
		outDir    string
		overwrite bool
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Bootstrap a .forge/tsd.yml from a named community blueprint.",
		Long: `forge templates init --from <id> writes a pre-filled .forge/tsd.yml into
the target directory. Edit the file to customise your project name, domain,
and stack choices, then run 'forge new' to scaffold.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if fromID == "" {
				return fmt.Errorf("--from is required; run 'forge templates list' to see available IDs")
			}

			// Validate template ID.
			content, ok := tsdTemplateContent[fromID]
			if !ok {
				return errcode.New(ErrUnknownTemplate,
					fmt.Sprintf("unknown template %q — run 'forge templates list' to see available IDs", fromID),
					nil,
				)
			}

			// Determine output path.
			if outDir == "" {
				outDir = "."
			}
			forgeDir := filepath.Join(outDir, ".forge")
			tsdPath := filepath.Join(forgeDir, "tsd.yml")

			if !overwrite {
				if _, err := os.Stat(tsdPath); err == nil {
					return errcode.New(ErrTSDExists,
						fmt.Sprintf("%s already exists; use --overwrite to replace it", tsdPath),
						nil,
					)
				}
			}

			if err := os.MkdirAll(forgeDir, 0o750); err != nil {
				return fmt.Errorf("create .forge directory: %w", err)
			}

			if err := os.WriteFile(tsdPath, []byte(content), 0o600); err != nil {
				return fmt.Errorf("write tsd.yml: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\nEdit it, then run: forge new \"<project-description>\"\n", tsdPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&fromID, "from", "", "template ID (see 'forge templates list')")
	cmd.Flags().StringVar(&outDir, "out", ".", "directory to write .forge/tsd.yml into")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "overwrite existing .forge/tsd.yml")
	return cmd
}
