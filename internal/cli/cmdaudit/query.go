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
package cmdaudit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/audit"
	"github.com/teragrid/forge/internal/errcode"
)

// ErrAuditQuery is returned when a query fails (e.g. bad --since format).
var ErrAuditQuery = errcode.Register(errcode.Code(3402), "audit query failed")

// newQueryCmd returns the `forge audit query` sub-subcommand.
func newQueryCmd() *cobra.Command {
	var (
		root     string
		verbF    string
		actionF  string
		sinceStr string
		limit    int
		asJSON   bool
	)
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Filter ledger entries by verb, action, or date.",
		Long: `Query reads .forge/audit.log and returns entries matching all
supplied filters (AND semantics). With no filters, all entries are returned.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrAuditQuery, "getwd", err)
				}
				root = cwd
			}
			var since time.Time
			if sinceStr != "" {
				t, err := time.Parse("2006-01-02", sinceStr)
				if err != nil {
					return errcode.Newf(ErrAuditQuery, err, "--since must be YYYY-MM-DD, got %q", sinceStr)
				}
				since = t
			}
			path := filepath.Join(root, audit.DefaultPath)
			ledger, err := audit.Open(path)
			if err != nil {
				return errcode.New(ErrAuditQuery, "open ledger", err)
			}
			all, err := ledger.All()
			if err != nil {
				return errcode.New(ErrAuditQuery, "read ledger", err)
			}
			var results []audit.Entry
			for _, e := range all {
				if verbF != "" && e.Verb != verbF {
					continue
				}
				if actionF != "" && e.Action != actionF {
					continue
				}
				if !since.IsZero() && e.Timestamp.Before(since) {
					continue
				}
				results = append(results, e)
				if limit > 0 && len(results) >= limit {
					break
				}
			}
			if asJSON {
				if results == nil {
					results = []audit.Entry{}
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "query: %d result(s)\n", len(results))
			for i, e := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "  #%d %s %s/%s hash=%s\n",
					i, e.Timestamp.Format("2006-01-02T15:04:05Z"), e.Verb, e.Action, short(e.Hash))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().StringVar(&verbF, "verb", "", "filter by verb name")
	cmd.Flags().StringVar(&actionF, "action", "", "filter by action name")
	cmd.Flags().StringVar(&sinceStr, "since", "", "include entries on/after YYYY-MM-DD")
	cmd.Flags().IntVar(&limit, "limit", 0, "max entries to return (0 = all)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}
