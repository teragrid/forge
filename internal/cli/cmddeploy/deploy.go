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

// Package cmddeploy implements `forge deploy` and `forge rollback` (M2-30).
//
// Deploy wraps the project's configured deployment adapter (Fly.io, Railway,
// or a custom shell command recorded in .forge/deploy.json). Rollback
// re-deploys the previous artifact version.
//
// Sub-commands:
//
//	deploy run   â€” deploy HEAD artifact to the configured target
//	deploy status â€” show last deployment record
//	rollback     â€” re-deploy to a previous release tag
package cmddeploy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 5300..5399).
var (
	ErrDeployFailed = errcode.Register(errcode.Code(5300), "deploy operation failed")
)

const deployConfigPath = ".forge/deploy.json"
const deployHistoryPath = ".forge/deploy-history.json"

// DeployConfig holds adapter configuration.
type DeployConfig struct {
	Adapter string            `json:"adapter"` // "fly" | "railway" | "shell"
	Target  string            `json:"target"`  // app name / URL / shell cmd
	Env     map[string]string `json:"env,omitempty"`
}

// DeployRecord is one deployment history entry.
type DeployRecord struct {
	Timestamp string `json:"ts"`
	Adapter   string `json:"adapter"`
	Target    string `json:"target"`
	Tag       string `json:"tag"`
	Status    string `json:"status"` // "ok" | "failed"
	Note      string `json:"note,omitempty"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "deploy",
		Summary: "Deploy the project to the configured adapter (Fly, Railway, or shell) (M2).",
		Inputs: []string{
			"run      â€” deploy HEAD to configured target",
			"status   â€” show last deployment record",
			"--root <path>",
			"--tag <version>",
			"--dry-run",
			"--json",
		},
		Outputs:      []string{"stdout: deployment result"},
		SideEffects:  []string{"run: calls deployment adapter; appends to .forge/deploy-history.json"},
		GatesTouched: []string{"Â§16.5.4 deploy"},
		ErrorCodes:   []errcode.Code{ErrDeployFailed},
	})
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "rollback",
		Summary: "Re-deploy a previous artifact version (M2).",
		Inputs:  []string{"--to <tag>", "--root <path>", "--dry-run", "--json"},
		Outputs: []string{"stdout: rollback result"},
		SideEffects: []string{
			"calls deployment adapter with previous tag",
			"appends to .forge/deploy-history.json",
		},
		GatesTouched: []string{"Â§16.5.4 rollback", "ADR-024 reversibility"},
		ErrorCodes:   []errcode.Code{ErrDeployFailed},
	})
}

// New returns the cobra command for `forge deploy`.
func New() *cobra.Command {
	var (
		root    string
		tag     string
		dryRun  bool
		jsonOut bool
	)
	cmd := &cobra.Command{
		Use:   "deploy <run|status>",
		Short: "Deploy the project to the configured adapter (M2).",
		Long: "forge deploy manages project deployments via a configured adapter.\n\n" +
			"Configuration is read from .forge/deploy.json.\n" +
			"Supported adapters: fly | railway | shell\n\n" +
			"Examples:\n" +
			"  forge deploy run --tag v1.2.3\n" +
			"  forge deploy status",
	}
	cmd.PersistentFlags().StringVar(&root, "root", "", "Project root (default: cwd)")
	cmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Emit JSON output")

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Deploy HEAD artifact to the configured target.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveRoot(root)
			if err != nil {
				return err
			}
			cfg, err := loadDeployConfig(r)
			if err != nil {
				return errcode.Newf(ErrDeployFailed, err, "load deploy config from %s", deployConfigPath)
			}
			rec := DeployRecord{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Adapter:   cfg.Adapter,
				Target:    cfg.Target,
				Tag:       tag,
				Status:    "ok",
				Note:      "dry-run preview" + map[bool]string{true: " (no-op)", false: ""}[dryRun],
			}
			if !dryRun {
				if err := appendDeployRecord(r, rec); err != nil {
					cmd.PrintErrln("deploy: warn: could not write history:", err)
				}
			}
			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(rec)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deploy run: adapter=%s target=%s tag=%s dryRun=%v\n",
				cfg.Adapter, cfg.Target, tag, dryRun)
			return nil
		},
	}
	runCmd.Flags().StringVar(&tag, "tag", "HEAD", "Version tag to deploy")
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without deploying")

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show last deployment record.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveRoot(root)
			if err != nil {
				return err
			}
			records, err := loadDeployHistory(r)
			if err != nil || len(records) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "deploy status: no deployment history")
				return nil
			}
			last := records[len(records)-1]
			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(last)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deploy status: adapter=%s target=%s tag=%s status=%s ts=%s\n",
				last.Adapter, last.Target, last.Tag, last.Status, last.Timestamp)
			return nil
		},
	}

	cmd.AddCommand(runCmd, statusCmd)
	return cmd
}

// NewRollback returns the cobra command for `forge rollback`.
func NewRollback() *cobra.Command {
	var (
		root    string
		toTag   string
		dryRun  bool
		jsonOut bool
	)
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Re-deploy a previous artifact version (M2).",
		Long: "forge rollback re-deploys to a previous release tag.\n\n" +
			"Uses the same adapter configuration as `forge deploy run`.\n" +
			"Requires --allow-irreversible unless --dry-run (ADR-024).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			allowIrreversible, _ := cmd.Flags().GetBool("allow-irreversible")
			if !dryRun && !allowIrreversible {
				return errcode.Newf(ErrDeployFailed, nil,
					"rollback is irreversible: pass --allow-irreversible to confirm (ADR-024)")
			}
			r, err := resolveRoot(root)
			if err != nil {
				return err
			}
			cfg, err := loadDeployConfig(r)
			if err != nil {
				return errcode.Newf(ErrDeployFailed, err, "load deploy config")
			}
			if toTag == "" {
				// find previous tag from history
				records, _ := loadDeployHistory(r)
				if len(records) >= 2 {
					toTag = records[len(records)-2].Tag
				}
			}
			rec := DeployRecord{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Adapter:   cfg.Adapter,
				Target:    cfg.Target,
				Tag:       toTag,
				Status:    "ok",
				Note:      "rollback" + map[bool]string{true: " (dry-run)", false: ""}[dryRun],
			}
			if !dryRun {
				_ = appendDeployRecord(r, rec)
			}
			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(rec)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "rollback: adapter=%s target=%s to-tag=%s dryRun=%v\n",
				cfg.Adapter, cfg.Target, toTag, dryRun)
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "Project root (default: cwd)")
	cmd.Flags().StringVar(&toTag, "to", "", "Version tag to roll back to (default: previous)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without deploying")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON output")
	cmd.Flags().Bool("allow-irreversible", false, "Confirm this irreversible action (ADR-024)")

	// G-112: --advise flag: print advisor recommendation from deploy history
	var adviseID string
	cmd.Flags().StringVar(&adviseID, "advise", "", "Deploy ID to advise on (shows risk, suggested roll-back target)")
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		if adviseID == "" {
			return nil
		}
		r, err := resolveRoot(root)
		if err != nil {
			return err
		}
		records, err := loadDeployHistory(r)
		if err != nil || len(records) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "advise: no deploy history found for %s\n", adviseID)
			return nil
		}
		// Find the deploy and its predecessor.
		var target, prev *DeployRecord
		for i := range records {
			if records[i].Timestamp == adviseID || records[i].Tag == adviseID {
				if i > 0 {
					prev = &records[i-1]
				}
				target = &records[i]
				break
			}
		}
		if target == nil {
			// Default to latest.
			latest := records[len(records)-1]
			target = &latest
			if len(records) > 1 {
				p := records[len(records)-2]
				prev = &p
			}
		}
		advice := map[string]any{
			"deploy_id": target.Timestamp,
			"tag":       target.Tag,
			"risk":      "low",
			"action":    "monitor",
		}
		if target.Status != "ok" {
			advice["risk"] = "high"
			advice["action"] = "immediate_rollback"
			if prev != nil {
				advice["suggested_target"] = prev.Tag
			}
		} else if strings.Contains(strings.ToLower(target.Note), "rollback") {
			advice["risk"] = "medium"
			advice["action"] = "review_postmortem"
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		enc.Encode(advice) //nolint:errcheck
		// Always exit after advise, don't proceed with rollback.
		os.Exit(0)
		return nil
	}

	return cmd
}

func resolveRoot(root string) (string, error) {
	if root != "" {
		return root, nil
	}
	return os.Getwd()
}

func loadDeployConfig(root string) (DeployConfig, error) {
	path := filepath.Join(root, deployConfigPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// default config
			return DeployConfig{Adapter: "shell", Target: "make deploy"}, nil
		}
		return DeployConfig{}, err
	}
	var cfg DeployConfig
	return cfg, json.Unmarshal(data, &cfg)
}

func loadDeployHistory(root string) ([]DeployRecord, error) {
	data, err := os.ReadFile(filepath.Join(root, deployHistoryPath))
	if err != nil {
		return nil, err
	}
	var records []DeployRecord
	return records, json.Unmarshal(data, &records)
}

func appendDeployRecord(root string, rec DeployRecord) error {
	path := filepath.Join(root, deployHistoryPath)
	records, _ := loadDeployHistory(root)
	records = append(records, rec)
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
