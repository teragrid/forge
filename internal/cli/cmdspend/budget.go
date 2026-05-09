// Package cmdspend implements `forge spend` — track and enforce LLM spend
// limits from .forge/llm-budget.json (DEV-M3-03).
package cmdspend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/llmbudget"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Error codes (range 2400..2499 — llm/budget).
var (
	ErrBudgetFailed   = errcode.Register(errcode.Code(2400), "budget operation failed")
	ErrBudgetExceeded = errcode.Register(errcode.Code(2401), "LLM spend limit exceeded")
	ErrBudgetInvalid  = errcode.Register(errcode.Code(2402), "invalid budget configuration")
)

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "spend",
		Summary: "Track and enforce LLM API spend limits (.forge/llm-budget.json).",
		Inputs: []string{
			"<subcommand>: status | set | reset",
			"--root <path> (default: cwd)",
			"--json",
		},
		Outputs:      []string{"stdout: spend summary or operation confirmation"},
		SideEffects:  []string{"`set` writes limits; `reset` clears history"},
		GatesTouched: []string{"§16.5.3 #3 — LLM budget enforcement"},
		ErrorCodes:   []errcode.Code{ErrBudgetFailed, ErrBudgetExceeded, ErrBudgetInvalid},
	})
}

// New returns the `forge spend` cobra command.
func New() *cobra.Command {
	var (
		root   string
		asJSON bool
	)

	cmd := &cobra.Command{
		Use:   "spend <status|set|reset>",
		Short: "Track and enforce LLM API spend limits.",
	}
	cmd.PersistentFlags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.PersistentFlags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")

	// ── status ────────────────────────────────────────────────────────────────
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show daily and monthly LLM spend vs configured limits.",
		RunE: func(c *cobra.Command, _ []string) error {
			b, _, err := openBudget(root)
			if err != nil {
				return errcode.New(ErrBudgetFailed, "load budget", err)
			}
			return renderStatus(c, b, asJSON)
		},
	})

	// ── set ───────────────────────────────────────────────────────────────────
	var dailyUSD, monthlyUSD float64
	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Set daily and/or monthly spend limits in USD.",
		RunE: func(c *cobra.Command, _ []string) error {
			b, path, err := openBudget(root)
			if err != nil {
				return errcode.New(ErrBudgetFailed, "load budget", err)
			}
			if err := b.SetLimits(dailyUSD, monthlyUSD); err != nil {
				return errcode.New(ErrBudgetInvalid, "set limits", err)
			}
			if err := b.Save(path); err != nil {
				return errcode.New(ErrBudgetFailed, "save budget", err)
			}
			fmt.Fprintf(c.OutOrStdout(),
				"budget: limits set (daily=$%.4f monthly=$%.4f)\n",
				b.Config.DailyLimitUSD, b.Config.MonthlyLimitUSD)
			return nil
		},
	}
	setCmd.Flags().Float64Var(&dailyUSD, "daily", 0, "daily spend limit in USD (0 = unlimited)")
	setCmd.Flags().Float64Var(&monthlyUSD, "monthly", 0, "monthly spend limit in USD (0 = unlimited)")
	cmd.AddCommand(setCmd)

	// ── reset ─────────────────────────────────────────────────────────────────
	var resetLimits bool
	resetCmd := &cobra.Command{
		Use:   "reset",
		Short: "Clear spend history (limits preserved unless --limits is set).",
		RunE: func(c *cobra.Command, _ []string) error {
			b, path, err := openBudget(root)
			if err != nil {
				return errcode.New(ErrBudgetFailed, "load budget", err)
			}
			b.Reset(resetLimits)
			if err := b.Save(path); err != nil {
				return errcode.New(ErrBudgetFailed, "save budget", err)
			}
			fmt.Fprintln(c.OutOrStdout(), "budget: history cleared")
			return nil
		},
	}
	resetCmd.Flags().BoolVar(&resetLimits, "limits", false, "also reset spend limits to zero")
	cmd.AddCommand(resetCmd)

	return cmd
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func openBudget(root string) (*llmbudget.Budget, string, error) {
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, "", err
		}
		root = cwd
	}
	path := filepath.Join(root, llmbudget.DefaultPath)
	b, err := llmbudget.Load(path)
	return b, path, err
}

// statusPayload is the JSON shape for `forge budget status --json`.
type statusPayload struct {
	DailySpendUSD   float64 `json:"daily_spend_usd"`
	MonthlySpendUSD float64 `json:"monthly_spend_usd"`
	DailyLimitUSD   float64 `json:"daily_limit_usd"`
	MonthlyLimitUSD float64 `json:"monthly_limit_usd"`
	RecordCount     int     `json:"record_count"`
}

func renderStatus(cmd *cobra.Command, b *llmbudget.Budget, asJSON bool) error {
	now := time.Now().UTC()
	p := statusPayload{
		DailySpendUSD:   b.DailySpend(now),
		MonthlySpendUSD: b.MonthlySpend(now),
		DailyLimitUSD:   b.Config.DailyLimitUSD,
		MonthlyLimitUSD: b.Config.MonthlyLimitUSD,
		RecordCount:     len(b.Records),
	}
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(p)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "LLM spend (%s)\n", now.Format("2006-01-02"))
	fmt.Fprintf(cmd.OutOrStdout(), "  today:   $%.4f", p.DailySpendUSD)
	if p.DailyLimitUSD > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), " / $%.4f limit", p.DailyLimitUSD)
	}
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintf(cmd.OutOrStdout(), "  month:   $%.4f", p.MonthlySpendUSD)
	if p.MonthlyLimitUSD > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), " / $%.4f limit", p.MonthlyLimitUSD)
	}
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintf(cmd.OutOrStdout(), "  records: %d\n", p.RecordCount)
	return nil
}
