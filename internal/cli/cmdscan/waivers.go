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

package cmdscan

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/waiver"
)

// ApplyWaivers removes from res every finding covered by a valid waiver under
// <root>/.forge/waivers/ and records how many were waived in res.Waived.
//
// Until now the waiver registry (internal/waiver, DEV-M1-17) was loaded and
// tested but never consulted by the scanner, so a repository had no in-tool way
// to accept a specific finding — the only options were to edit the offending
// code or to live with a permanently red gate.
//
// Rules, in the order they are enforced:
//
//  1. A waiver must state why (rationale), who approved it (approved_by) and
//     when it lapses (expires_at). A waiver missing any of them is an error, not
//     a silent no-op: the scan fails closed so an unreviewable exemption cannot
//     hide findings.
//  2. A finding matching an expired waiver is NOT suppressed, and the expiry is
//     reported in res.Note so it is visible why the finding came back.
//  3. A waiver with no file_path covers its rule everywhere; with file_path it
//     covers that one file (matched on the scan-relative, slash-separated path).
//
// With no waivers directory this is a no-op.
func ApplyWaivers(root string, res *ScanResult) error {
	reg, err := waiver.LoadDefault(root)
	if err != nil {
		return errcode.New(ErrScanFailed, "load waivers", err)
	}
	if len(reg.All()) == 0 {
		return nil
	}
	if err := validateWaivers(reg.All()); err != nil {
		return errcode.New(ErrScanFailed, "invalid waiver", err)
	}

	kept := make([]Finding, 0, len(res.Findings))
	expired := map[string]struct{}{}
	for _, f := range res.Findings {
		ok, werr := reg.IsWaived(f.Rule, filepath.ToSlash(f.File))
		switch {
		case werr != nil && errors.Is(werr, waiver.ErrWaiverExpired):
			expired[werr.Error()] = struct{}{}
			kept = append(kept, f)
		case werr != nil:
			return errcode.New(ErrScanFailed, "evaluate waiver", werr)
		case ok:
			res.Waived++
		default:
			kept = append(kept, f)
		}
	}
	res.Findings = kept
	finalizeStatus(res)

	if res.Waived > 0 {
		res.Note = joinNote(res.Note, fmt.Sprintf("%d finding(s) waived by .forge/waivers", res.Waived))
	}
	if len(expired) > 0 {
		msgs := make([]string, 0, len(expired))
		for m := range expired {
			msgs = append(msgs, m)
		}
		sort.Strings(msgs)
		res.Note = joinNote(res.Note, "expired waiver(s) NOT honoured: "+strings.Join(msgs, "; "))
	}
	return nil
}

// validateWaivers rejects waivers that omit the fields the waiver package
// documents as required. It reports every problem at once.
func validateWaivers(specs []waiver.WaiverSpec) error {
	var problems []string
	for i, w := range specs {
		id := w.ID
		if id == "" {
			id = fmt.Sprintf("#%d", i+1)
		}
		var missing []string
		if w.RuleID == "" {
			missing = append(missing, "rule_id")
		}
		if strings.TrimSpace(w.Rationale) == "" {
			missing = append(missing, "rationale")
		}
		if strings.TrimSpace(w.ApprovedBy) == "" {
			missing = append(missing, "approved_by")
		}
		if strings.TrimSpace(w.ExpiresAt) == "" {
			missing = append(missing, "expires_at")
		}
		if len(missing) > 0 {
			problems = append(problems, fmt.Sprintf("waiver %s is missing %s", id, strings.Join(missing, ", ")))
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func joinNote(existing, add string) string {
	if existing == "" {
		return add
	}
	return existing + " | " + add
}
