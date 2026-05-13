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
// Package cmdplugin implements `forge plugin` (M2.x).
// Subcommands:
//   - list    — enumerate registered plugins (text or JSON)
//   - show    — inspect a single plugin's manifest
//   - install — record a plugin in the project lock file
//   - upgrade — update a pinned plugin version in the lock file
//   - remove  — remove a plugin from the lock file
//
// In-process (in-tree) plugins are registered via init() in their owning
// packages (e.g. cmdscan, cmdupgrade). The wazero-backed WASM loader will
// register dynamic plugins here in M2.2.
package cmdplugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/plugin"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 3500..3599).
var (
	ErrPluginUnknown       = errcode.Register(errcode.Code(3500), "unknown plugin")
	ErrPluginUsage         = errcode.Register(errcode.Code(3501), "plugin command usage error")
	ErrPluginInstallFailed = errcode.Register(errcode.Code(3502), "plugin install failed")
	ErrPluginLockInvalid   = errcode.Register(errcode.Code(3503), "plugin lock file invalid or conflict")
)

// lockFilePath is the project-relative path to the plugin lock file.
const lockFilePath = ".forge/plugins.json"

// LockEntry records a single pinned plugin.
type LockEntry struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  string `json:"source"`           // "in-tree" or URL
	SHA256  string `json:"sha256,omitempty"` // future integrity check
}

// LockFile is the top-level lock structure.
type LockFile struct {
	Plugins []LockEntry `json:"plugins"`
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "plugin",
		Summary: "Inspect and manage Forge plugins (in-tree scanners, codemods, future WASM).",
		Inputs: []string{
			"<subcommand>: list | show | install | upgrade | remove",
			"--kind <scanner|codemod|provider|template> (filter, list only)",
			"--root <path> (project root; install/upgrade/remove only)",
			"--json (machine-readable output)",
		},
		Outputs:      []string{"stdout: plugin table or JSON"},
		SideEffects:  []string{"install/upgrade/remove mutate .forge/plugins.json"},
		GatesTouched: []string{"§16.5.4 #5 — supply-chain visibility"},
		ErrorCodes:   []errcode.Code{ErrPluginUnknown, ErrPluginUsage, ErrPluginInstallFailed, ErrPluginLockInvalid},
	})
}

// New returns the root cobra command for `forge plugin`.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Inspect and manage Forge plugins (M2.x).",
	}
	cmd.AddCommand(newListCmd(), newShowCmd(), newInstallCmd(), newUpgradeCmd(), newRemoveCmd(),
		newSearchCmd(), newDocsCmd())
	return cmd
}

func newListCmd() *cobra.Command {
	var (
		asJSON   bool
		kindFlag string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered plugins.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			plugins := plugin.Default().All()
			if kindFlag != "" {
				k := plugin.Kind(kindFlag)
				switch k {
				case plugin.KindScanner, plugin.KindCodemod, plugin.KindProvider, plugin.KindTemplate:
				default:
					return errcode.Newf(ErrPluginUsage, nil,
						"unknown --kind %q; one of: scanner, codemod, provider, template", kindFlag)
				}
				plugins = plugin.Default().ByKind(k)
			}

			if asJSON {
				out := make([]plugin.Manifest, 0, len(plugins))
				for _, p := range plugins {
					out = append(out, p.Manifest())
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tKIND\tVERSION\tSUMMARY")
			for _, p := range plugins {
				m := p.Manifest()
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.Name, m.Kind, m.Version, m.Summary)
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	cmd.Flags().StringVar(&kindFlag, "kind", "", "filter by kind (scanner|codemod|provider|template)")
	return cmd
}

func newShowCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show a plugin's manifest.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			p, ok := plugin.Default().Lookup(name)
			if !ok {
				return errcode.Newf(ErrPluginUnknown, nil, "no plugin named %q", name)
			}
			m := p.Manifest()
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(m)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "name:    %s\n", m.Name)
			fmt.Fprintf(w, "kind:    %s\n", m.Kind)
			fmt.Fprintf(w, "version: %s\n", m.Version)
			if m.Author != "" {
				fmt.Fprintf(w, "author:  %s\n", m.Author)
			}
			if m.Summary != "" {
				fmt.Fprintf(w, "summary: %s\n", m.Summary)
			}
			if m.Forge != "" {
				fmt.Fprintf(w, "forge:   %s\n", m.Forge)
			}
			if len(m.Capabilities) > 0 {
				fmt.Fprintf(w, "caps:    %s\n", strings.Join(m.Capabilities, ", "))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

// ── install / upgrade / remove ────────────────────────────────────────────

func newInstallCmd() *cobra.Command {
	var root string
	cmd := &cobra.Command{
		Use:   "install <name[@version]>",
		Short: "Record a plugin in the project lock file.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if root == "" {
				var err error
				root, err = os.Getwd()
				if err != nil {
					return errcode.New(ErrPluginInstallFailed, "getwd", err)
				}
			}
			name, version := splitNameVersion(args[0])
			if name == "" {
				return errcode.New(ErrPluginUsage, "plugin name must not be empty", nil)
			}
			lf, err := readLock(root)
			if err != nil {
				return err
			}
			for _, e := range lf.Plugins {
				if e.Name == name {
					if version != "" && e.Version == version {
						// Same explicit version already pinned → no-op.
						fmt.Fprintf(cmd.OutOrStdout(), "plugin %q already installed (version %s)\n", name, e.Version)
						return nil
					}
					if version == "" {
						// Plugin is already pinned; caller must specify explicit version to change it.
						return errcode.Newf(ErrPluginLockInvalid, nil,
							"plugin %q is already pinned to %s; specify an explicit version to upgrade", name, e.Version)
					}
					// Different explicit version → fall through to add (replace).
				}
			}
			ver := version
			if ver == "" {
				ver = "latest"
			}
			lf.Plugins = append(lf.Plugins, LockEntry{Name: name, Version: ver, Source: "in-tree"})
			if err := writeLock(root, lf); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "installed plugin %q version %s\n", name, ver)
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	return cmd
}

func newUpgradeCmd() *cobra.Command {
	var (
		root    string
		version string
	)
	cmd := &cobra.Command{
		Use:   "upgrade <name>",
		Short: "Update a pinned plugin to a new version.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if root == "" {
				var err error
				root, err = os.Getwd()
				if err != nil {
					return errcode.New(ErrPluginInstallFailed, "getwd", err)
				}
			}
			name := strings.TrimSpace(args[0])
			lf, err := readLock(root)
			if err != nil {
				return err
			}
			found := false
			for i, e := range lf.Plugins {
				if e.Name == name {
					newVer := version
					if newVer == "" {
						newVer = "latest"
					}
					lf.Plugins[i].Version = newVer
					found = true
					break
				}
			}
			if !found {
				return errcode.Newf(ErrPluginUnknown, nil,
					"plugin %q not found in lock file; run 'forge plugin install %s' first", name, name)
			}
			if err := writeLock(root, lf); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "upgraded plugin %q to version %s\n", name, version)
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	cmd.Flags().StringVar(&version, "version", "", "target version (default: latest)")
	return cmd
}

func newRemoveCmd() *cobra.Command {
	var root string
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a plugin from the lock file.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if root == "" {
				var err error
				root, err = os.Getwd()
				if err != nil {
					return errcode.New(ErrPluginInstallFailed, "getwd", err)
				}
			}
			name := strings.TrimSpace(args[0])
			lf, err := readLock(root)
			if err != nil {
				return err
			}
			orig := len(lf.Plugins)
			filtered := lf.Plugins[:0]
			for _, e := range lf.Plugins {
				if e.Name != name {
					filtered = append(filtered, e)
				}
			}
			if len(filtered) == orig {
				return errcode.Newf(ErrPluginUnknown, nil,
					"plugin %q not found in lock file", name)
			}
			lf.Plugins = filtered
			if err := writeLock(root, lf); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed plugin %q from lock file\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: cwd)")
	return cmd
}

// ── lock file helpers ─────────────────────────────────────────────────────

// readLock reads the plugin lock file from <root>/.forge/plugins.json.
// Returns an empty LockFile if the file does not exist.
func readLock(root string) (LockFile, error) {
	path := filepath.Join(root, lockFilePath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return LockFile{Plugins: []LockEntry{}}, nil
		}
		return LockFile{}, errcode.Newf(ErrPluginLockInvalid, err, "read lock file %s", path)
	}
	var lf LockFile
	if err := json.Unmarshal(data, &lf); err != nil {
		return LockFile{}, errcode.Newf(ErrPluginLockInvalid, err, "parse lock file %s", path)
	}
	if lf.Plugins == nil {
		lf.Plugins = []LockEntry{}
	}
	return lf, nil
}

// writeLock persists the lock file to <root>/.forge/plugins.json.
func writeLock(root string, lf LockFile) error {
	dir := filepath.Join(root, ".forge")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return errcode.Newf(ErrPluginInstallFailed, err, "create .forge dir")
	}
	path := filepath.Join(root, lockFilePath)
	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return errcode.Newf(ErrPluginInstallFailed, err, "marshal lock file")
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return errcode.Newf(ErrPluginInstallFailed, err, "write lock file %s", path)
	}
	return nil
}

// splitNameVersion splits "name@version" → ("name","version").
// If no "@" is present, version is "".
func splitNameVersion(s string) (name, version string) {
	if i := strings.LastIndex(s, "@"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// newSearchCmd implements `forge plugin search <query>` (spec §4 plugin sub-verb).
// Full registry search is planned for M2; M1 returns an informative stub.
func newSearchCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the Forge plugin registry for plugins matching a query (M2).",
		Long: "Searches the Forge plugin registry (registry.forgeframework.dev) for plugins\n" +
			"matching the given keyword or tag.\n\n" +
			"Note: remote registry search is scheduled for M2. " +
			"In M1, only locally registered (in-tree) plugins can be found via `forge plugin list`.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(args[0])
			// M1 stub: search local registry only.
			plugins := plugin.Default().All()
			var matches []plugin.Plugin
			for _, p := range plugins {
				m := p.Manifest()
				if strings.Contains(strings.ToLower(m.Name), strings.ToLower(query)) ||
					strings.Contains(strings.ToLower(m.Summary), strings.ToLower(query)) {
					matches = append(matches, p)
				}
			}
			if asJSON {
				out := make([]plugin.Manifest, 0, len(matches))
				for _, p := range matches {
					out = append(out, p.Manifest())
				}
				if out == nil {
					out = []plugin.Manifest{}
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			if len(matches) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no local plugins matched %q (remote registry search available in M2)\n", query)
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tKIND\tVERSION\tSUMMARY")
			for _, p := range matches {
				m := p.Manifest()
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.Name, m.Kind, m.Version, m.Summary)
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

// newDocsCmd implements `forge plugin docs <name>` (spec §4 plugin sub-verb).
func newDocsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs <name>",
		Short: "Show documentation for a plugin.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			p, ok := plugin.Default().Lookup(name)
			if !ok {
				return errcode.Newf(ErrPluginUnknown, nil,
					"no plugin named %q; run 'forge plugin list' to see available plugins", name)
			}
			m := p.Manifest()
			fmt.Fprintf(cmd.OutOrStdout(), "plugin: %s\n", m.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "kind:   %s\n", m.Kind)
			fmt.Fprintf(cmd.OutOrStdout(), "ver:    %s\n", m.Version)
			if m.Author != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "author: %s\n", m.Author)
			}
			if m.Summary != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", m.Summary)
			}
			return nil
		},
	}
	return cmd
}
