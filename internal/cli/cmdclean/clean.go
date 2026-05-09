// Package cmdclean implements `forge clean` (DEV-M0-15).
//
// Walks the project root, classifies each tracked path against the
// .forge/manifest scratch + managed pattern lists, and either reports
// (default / --check) or deletes (--apply) the candidates.
package cmdclean

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/manifest"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 1300..1399).
var (
	ErrCleanFound  = errcode.Register(errcode.Code(1300), "clean found unmanaged scratch files")
	ErrCleanFailed = errcode.Register(errcode.Code(1301), "clean apply failed")
)

// Result is the summary of a clean run.
type Result struct {
	Root         string   `json:"root"`
	ManifestPath string   `json:"manifest_path"`
	Mode         string   `json:"mode"` // "check" or "apply"
	Candidates   []string `json:"candidates"`
	Deleted      []string `json:"deleted,omitempty"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "clean",
		Summary: "Find and (optionally) remove LLM-scratch and other unmanaged files.",
		Inputs: []string{
			"--check (default; report only, exit 1 if anything found)",
			"--apply (delete the candidates)",
			"--root <path> (project root; default cwd)",
			"--json (machine-readable output)",
		},
		Outputs:      []string{"stdout: candidate list (text or JSON)"},
		SideEffects:  []string{"--apply deletes files matching the manifest [scratch] section"},
		GatesTouched: []string{"§16.5.4 #11 — repo hygiene"},
		ErrorCodes:   []errcode.Code{ErrCleanFound, ErrCleanFailed},
	})
}

// New returns the cobra command.
func New() *cobra.Command {
	var (
		root   string
		check  bool
		apply  bool
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Find/remove unmanaged scratch files.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if apply && check {
				return errcode.New(ErrCleanFailed, "use either --check or --apply, not both", nil)
			}
			if !apply {
				check = true // default
			}
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errcode.New(ErrCleanFailed, "getwd", err)
				}
				root = cwd
			}

			res, err := Run(root, apply)
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(res); err != nil {
					return err
				}
			} else {
				renderText(cmd, res)
			}

			if !apply && len(res.Candidates) > 0 {
				return errcode.Newf(ErrCleanFound, nil,
					"%d candidate(s) found; rerun with --apply to delete", len(res.Candidates))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().BoolVar(&check, "check", false, "report only (default)")
	cmd.Flags().BoolVar(&apply, "apply", false, "delete found candidates")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

// Run scans root and (optionally) deletes scratch candidates. Exposed for
// tests + future ship-checkpoint integration.
func Run(root string, apply bool) (*Result, error) {
	mfPath := filepath.Join(root, manifest.DefaultPath)
	mf, err := manifest.Load(mfPath)
	if err != nil {
		return nil, err
	}

	mode := "check"
	if apply {
		mode = "apply"
	}
	res := &Result{Root: root, ManifestPath: mf.Path, Mode: mode}

	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if p == root {
			return nil
		}
		// Always skip .git for safety.
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if mf.IsScratch(rel) {
			res.Candidates = append(res.Candidates, rel)
			if d.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	if walkErr != nil {
		return nil, errcode.New(ErrCleanFailed, "walk", walkErr)
	}
	sort.Strings(res.Candidates)

	if apply {
		for _, c := range res.Candidates {
			full := filepath.Join(root, c)
			if err := os.RemoveAll(full); err != nil {
				return nil, errcode.Newf(ErrCleanFailed, err, "remove %s", c)
			}
			res.Deleted = append(res.Deleted, c)
		}
	}
	return res, nil
}

func renderText(cmd *cobra.Command, r *Result) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "forge clean (%s) root=%s\n", r.Mode, r.Root)
	fmt.Fprintf(w, "manifest: %s\n", r.ManifestPath)
	if len(r.Candidates) == 0 {
		fmt.Fprintln(w, "no candidates found.")
		return
	}
	fmt.Fprintf(w, "%d candidate(s):\n", len(r.Candidates))
	for _, c := range r.Candidates {
		marker := "would delete"
		if r.Mode == "apply" {
			marker = "deleted    "
		}
		fmt.Fprintf(w, "  %s  %s\n", marker, c)
	}
	if !strings.EqualFold(r.Mode, "apply") {
		fmt.Fprintln(w, "\nrerun with --apply to delete.")
	}
}
