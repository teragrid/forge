// Package cmdtelemetry implements `forge telemetry` — opt-in / opt-out for
// file-based observability spans (ADR-006, DEV-M3-01).
package cmdtelemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/telemetry"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Error codes (range 4100..4199 — cli/telemetry).
var (
	ErrTelemetryFailed = errcode.Register(errcode.Code(4100), "telemetry operation failed")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "telemetry",
		Summary: "Manage opt-in telemetry (ADR-006). Subcommands: enable, disable, status, rotate-id.",
		Inputs: []string{
			"<subcommand>: enable | disable | status | rotate-id",
			"--root <path> (default: cwd)",
			"--json",
		},
		Outputs:      []string{"stdout: current opt-in state"},
		SideEffects:  []string{".forge/telemetry.json (enable/disable/rotate-id modify this file)"},
		GatesTouched: []string{"§16.5.3 #5 — telemetry consent / ADR-006"},
		ErrorCodes:   []errcode.Code{ErrTelemetryFailed},
	})
}

// New returns the `forge telemetry` cobra command.
func New() *cobra.Command {
	var root string
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "telemetry <enable|disable|status|rotate-id>",
		Short: "Manage opt-in telemetry spans (ADR-006).",
	}
	cmd.PersistentFlags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.PersistentFlags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")

	// ── enable ────────────────────────────────────────────────────────────────
	cmd.AddCommand(&cobra.Command{
		Use:   "enable",
		Short: "Opt in to telemetry span collection.",
		RunE: func(c *cobra.Command, _ []string) error {
			cfg, path, err := loadCfg(root)
			if err != nil {
				return errcode.New(ErrTelemetryFailed, "load", err)
			}
			cfg.Enabled = true
			if err := telemetry.SaveConfig(path, cfg); err != nil {
				return errcode.New(ErrTelemetryFailed, "save", err)
			}
			printStatus(c, cfg, asJSON)
			return nil
		},
	})

	// ── disable ───────────────────────────────────────────────────────────────
	cmd.AddCommand(&cobra.Command{
		Use:   "disable",
		Short: "Opt out of telemetry span collection.",
		RunE: func(c *cobra.Command, _ []string) error {
			cfg, path, err := loadCfg(root)
			if err != nil {
				return errcode.New(ErrTelemetryFailed, "load", err)
			}
			cfg.Enabled = false
			if err := telemetry.SaveConfig(path, cfg); err != nil {
				return errcode.New(ErrTelemetryFailed, "save", err)
			}
			printStatus(c, cfg, asJSON)
			return nil
		},
	})

	// ── status ────────────────────────────────────────────────────────────────
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show current telemetry opt-in status.",
		RunE: func(c *cobra.Command, _ []string) error {
			cfg, _, err := loadCfg(root)
			if err != nil {
				return errcode.New(ErrTelemetryFailed, "load", err)
			}
			printStatus(c, cfg, asJSON)
			return nil
		},
	})

	// ── rotate-id ─────────────────────────────────────────────────────────────
	cmd.AddCommand(&cobra.Command{
		Use:   "rotate-id",
		Short: "Generate a new random install ID (pseudonym rotation).",
		RunE: func(c *cobra.Command, _ []string) error {
			cfg, path, err := loadCfg(root)
			if err != nil {
				return errcode.New(ErrTelemetryFailed, "load", err)
			}
			telemetry.RotateInstallID(cfg)
			if err := telemetry.SaveConfig(path, cfg); err != nil {
				return errcode.New(ErrTelemetryFailed, "save", err)
			}
			printStatus(c, cfg, asJSON)
			return nil
		},
	})

	return cmd
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func loadCfg(root string) (*telemetry.Config, string, error) {
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, "", err
		}
		root = cwd
	}
	path := filepath.Join(root, telemetry.DefaultConfigPath)
	cfg, err := telemetry.LoadConfig(path)
	return cfg, path, err
}

type statusPayload struct {
	Enabled   bool   `json:"enabled"`
	InstallID string `json:"install_id"`
}

func printStatus(cmd *cobra.Command, cfg *telemetry.Config, asJSON bool) {
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		enc.Encode(statusPayload{Enabled: cfg.Enabled, InstallID: cfg.InstallID}) //nolint:errcheck
		return
	}
	state := "disabled"
	if cfg.Enabled {
		state = "enabled"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "telemetry: %s  install_id=%s\n", state, cfg.InstallID)
}
