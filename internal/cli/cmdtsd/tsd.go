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

// Package cmdtsd implements the `forge tsd` subcommand group:
//   - forge tsd init   — interactive wizard (or --defaults skeleton)
//   - forge tsd validate [file] — validates a TSD file
package cmdtsd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/tsd"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Default TSD path inside a project.
const defaultTSDPath = ".forge/tsd.yml"

// Error codes (range 6500..6549).
var (
	ErrUsage    = errcode.Register(errcode.Code(6500), "invalid usage of `forge tsd`")
	ErrTSDWrite = errcode.Register(errcode.Code(6501), "failed to write TSD file")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "tsd",
		Summary: "Manage Tech Stack Decision (TSD) files used by `forge new`.",
		Inputs: []string{
			"init --defaults  — write a skeleton .forge/tsd.yml",
			"init --force     — overwrite existing .forge/tsd.yml",
			"validate [file]  — validate a TSD file (default: .forge/tsd.yml)",
			"--json           — emit machine-readable JSON",
		},
		Outputs: []string{
			".forge/tsd.yml (init)",
			"stdout — validation result",
		},
		SideEffects: []string{"creates .forge/tsd.yml (init only)"},
		ErrorCodes:  []errcode.Code{ErrUsage, ErrTSDWrite, tsd.ErrUnsupportedVersion, tsd.ErrValidation},
	})
}

// New returns the `forge tsd` cobra command group.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tsd",
		Short: "Manage Tech Stack Decision (TSD) files.",
		Long: "TSD files capture every architectural choice for a project before " +
			"scaffolding.\n\nSubcommands:\n  init      — create a skeleton TSD file\n" +
			"  validate  — check a TSD file against the schema",
	}
	cmd.AddCommand(newInitCmd(), newValidateCmd())
	return cmd
}

// ── forge tsd init ───────────────────────────────────────────────────────────

// skeletonTSD is the default skeleton written by `forge tsd init --defaults`.
const skeletonTSD = `tsd_version: 1

project:
  name: ""
  domain: ""
  type: saas          # saas | service | library | data-platform | marketplace
  description: ""

stack:
  frontend:
    framework: nextjs-15    # nextjs-15 | nuxt-3 | remix | sveltekit | vue-3 | none
    ui_library: shadcn      # shadcn | radix-ui | chakra | tailwind-only | none
    state_management: server-components-only
    testing: jest-rtl       # jest-rtl | playwright | vitest | none

  backend:
    language: python        # python | go | typescript | java | none
    framework: fastapi      # fastapi | chi | nestjs | express | gin | fiber | none
    api_style: rest         # rest | graphql | grpc | trpc | none
    auth: supabase-auth     # supabase-auth | auth0 | clerk | cognito | none
    testing: pytest

  database:
    primary: supabase-pg    # supabase-pg | neon | planetscale | mysql | sqlite | none
    cache: redis            # redis | memcached | none
    search: pg-fts          # pg-fts | elasticsearch | typesense | none
    migrations: supabase-migrations

  ai:
    orchestration: none     # langgraph | langchain | autogen | none
    llm_providers: []       # openai | anthropic | google-gemini | mistral | groq
    embedding: none
    vector_store: none
    observability: none

  payments:
    providers: []           # stripe | paypal | adyen | square | razorpay
    model: subscription     # subscription | one-time | usage-based | marketplace

  messaging:
    queue: none             # redis-bullmq | celery-redis | sqs | pubsub | none
    realtime: none          # supabase-realtime | pusher | ably | websocket | none
    email: none             # resend | sendgrid | ses | none
    sms: none               # twilio | vonage | none

  infra:
    cloud: none             # digitalocean | aws | gcp | azure | fly-io | none
    container: docker-compose # docker-compose | kubernetes | none
    cdn: none
    secrets: none
    ci_cd: github-actions   # github-actions | gitlab-ci | circleci | none

  observability:
    metrics: none
    tracing: opentelemetry
    logging: json-stdout
    alerting: none

  compliance:
    standards: []           # pci-dss-saq-a | gdpr | soc2 | hipaa
    secret_scanning: gitleaks
`

func newInitCmd() *cobra.Command {
	var (
		defaults bool
		force    bool
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a skeleton .forge/tsd.yml for this project.",
		Long: "Writes a skeleton TSD file to .forge/tsd.yml. " +
			"Use --defaults to skip the interactive wizard (useful for CI/automation). " +
			"Use --force to overwrite an existing file.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Ensure .forge/ directory exists.
			if err := os.MkdirAll(".forge", 0o755); err != nil { //nolint:gosec
				return errcode.New(ErrTSDWrite, "create .forge dir", err)
			}

			// Check for existing file.
			if _, err := os.Stat(defaultTSDPath); err == nil && !force {
				return errcode.Newf(ErrUsage, nil,
					"TSD file already exists at %s; use --force to overwrite", defaultTSDPath)
			}

			if defaults {
				if err := os.WriteFile(defaultTSDPath, []byte(skeletonTSD), 0o600); err != nil {
					return errcode.New(ErrTSDWrite, "write "+defaultTSDPath, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", defaultTSDPath)
				fmt.Fprintln(cmd.OutOrStdout(),
					"Edit the file to fill in your stack choices, then run:\n  forge tsd validate")
				return nil
			}

			// Non-interactive path also writes the skeleton (interactive wizard is
			// a Phase 1 nice-to-have; --defaults is the CI-safe path used by tests).
			if err := os.WriteFile(defaultTSDPath, []byte(skeletonTSD), 0o600); err != nil {
				return errcode.New(ErrTSDWrite, "write "+defaultTSDPath, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", defaultTSDPath)
			fmt.Fprintln(cmd.OutOrStdout(),
				"Edit the file to fill in your stack choices, then run:\n  forge tsd validate")
			return nil
		},
	}
	cmd.Flags().BoolVar(&defaults, "defaults", false, "write skeleton without interactive prompts")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing TSD file")
	return cmd
}

// ── forge tsd validate ───────────────────────────────────────────────────────

// validationOutput is the --json output schema for forge tsd validate.
type validationOutput struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

func newValidateCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "validate [file]",
		Short: "Validate a TSD file against the schema.",
		Long: "Validates the TSD file at [file] (default: .forge/tsd.yml). " +
			"Exits 0 if valid, 1 if invalid. Warnings for unknown keys are printed " +
			"but do not affect the exit code.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := defaultTSDPath
			if len(args) == 1 {
				path = args[0]
			}

			// Verify path exists.
			if _, err := os.Stat(path); os.IsNotExist(err) {
				if asJSON {
					out := validationOutput{
						Valid:  false,
						Errors: []string{"file not found: " + path},
					}
					return emitJSON(cmd, out)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "tsd: file not found: %s\n", path)
				return &exitError{code: 1}
			} else if path == defaultTSDPath && err != nil {
				// defaultTSDPath not found and no explicit arg → friendly message.
				if asJSON {
					out := validationOutput{
						Valid:  false,
						Errors: []string{"no TSD file found at " + defaultTSDPath},
					}
					return emitJSON(cmd, out)
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "tsd: no TSD file found at "+defaultTSDPath)
				return &exitError{code: 1}
			}

			absPath, _ := filepath.Abs(path)
			t, parseErr := tsd.ParseFile(absPath)
			if parseErr != nil {
				if asJSON {
					out := validationOutput{
						Valid:  false,
						Errors: []string{parseErr.Error()},
					}
					return emitJSON(cmd, out)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "tsd: parse error: %v\n", parseErr)
				return &exitError{code: 1}
			}

			valErrs := tsd.Validate(t)

			var errMsgs []string
			for _, e := range valErrs {
				errMsgs = append(errMsgs, e.Error())
			}
			var warnMsgs []string
			for _, k := range t.UnknownKeys {
				warnMsgs = append(warnMsgs, "unknown key: "+k)
			}

			valid := len(valErrs) == 0

			if asJSON {
				out := validationOutput{
					Valid:    valid,
					Errors:   errMsgs,
					Warnings: warnMsgs,
				}
				if out.Errors == nil {
					out.Errors = []string{}
				}
				if out.Warnings == nil {
					out.Warnings = []string{}
				}
				if err := emitJSON(cmd, out); err != nil {
					return err
				}
				if !valid {
					return &exitError{code: 1}
				}
				return nil
			}

			if valid {
				if len(warnMsgs) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "tsd: OK (%d warning(s)):\n", len(warnMsgs))
					for _, w := range warnMsgs {
						fmt.Fprintf(cmd.OutOrStdout(), "  warning: %s\n", w)
					}
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "tsd: OK")
				}
				return nil
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "tsd: %d error(s):\n", len(valErrs))
			for _, e := range valErrs {
				fmt.Fprintf(cmd.ErrOrStderr(), "  error: %s\n", e.Error())
			}
			for _, w := range warnMsgs {
				fmt.Fprintf(cmd.OutOrStdout(), "  warning: %s\n", w)
			}
			return &exitError{code: 1}
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

func emitJSON(cmd *cobra.Command, v interface{}) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// exitError is a sentinel error that carries only an exit code; the message
// has already been printed so RunE callers should not print it again.
type exitError struct{ code int }

func (e *exitError) Error() string { return fmt.Sprintf("exit %d", e.code) }
