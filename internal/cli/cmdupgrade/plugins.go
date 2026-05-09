// Package cmdupgrade: in-tree plugin adapters for codemods.
//
// Every codemod registered with codemod.Default() is exposed as a
// plugin.Codemod and registered with plugin.Default() at init() time so that
// `forge plugin list` discovers codemods alongside scanners.
//
// Adapter only — codemod.Default() remains the source of truth.
package cmdupgrade

import (
	"context"

	"github.com/teragrid/forge/internal/codemod"
	"github.com/teragrid/forge/internal/plugin"
)

type codemodAdapter struct {
	manifest plugin.Manifest
	inner    codemod.Codemod
}

func (c *codemodAdapter) Manifest() plugin.Manifest { return c.manifest }

func (c *codemodAdapter) Apply(_ context.Context, root string, dryRun bool) (plugin.Result, error) {
	rep, err := c.inner.Apply(root, dryRun)
	if err != nil {
		return plugin.Result{}, err
	}
	return plugin.Result{
		Files:   rep.Files,
		Changed: rep.Changed,
		DryRun:  rep.DryRun,
		Detail:  rep.Detail,
	}, nil
}

func init() {
	reg := plugin.Default()
	for _, c := range codemod.Default().All() {
		name := c.Name()
		if _, ok := reg.Lookup(name); ok {
			continue
		}
		reg.Register(&codemodAdapter{
			manifest: plugin.Manifest{
				Name: name, Version: "1.0.0", Kind: plugin.KindCodemod,
				Author: "forge", Summary: c.Description(),
			},
			inner: c,
		})
	}
}
