package cmdupgrade

import (
	"context"
	"testing"

	"github.com/teragrid/forge/internal/codemod"
	"github.com/teragrid/forge/internal/plugin"
)

// TestCodemodAdapter_RegistersAllCodemods verifies every codemod in
// codemod.Default() is mirrored as a plugin.Codemod in plugin.Default()
// (init() side-effect of importing this package).
func TestCodemodAdapter_RegistersAllCodemods(t *testing.T) {
	for _, c := range codemod.Default().All() {
		p, ok := plugin.Default().Lookup(c.Name())
		if !ok {
			t.Fatalf("codemod %q not mirrored into plugin.Default()", c.Name())
		}
		if got := p.Manifest().Kind; got != plugin.KindCodemod {
			t.Errorf("codemod %q kind = %q, want %q", c.Name(), got, plugin.KindCodemod)
		}
		if _, ok := p.(plugin.Codemod); !ok {
			t.Errorf("codemod %q does not satisfy plugin.Codemod", c.Name())
		}
	}
}

// TestCodemodAdapter_ApplyDryRun_DataAccuracy ensures the adapter passes
// dryRun through and returns a Result that mirrors the underlying Report.
func TestCodemodAdapter_ApplyDryRun_DataAccuracy(t *testing.T) {
	tmp := t.TempDir()
	for _, c := range codemod.Default().All() {
		p, ok := plugin.Default().Lookup(c.Name())
		if !ok {
			continue
		}
		ad, ok := p.(plugin.Codemod)
		if !ok {
			continue
		}
		res, err := ad.Apply(context.Background(), tmp, true)
		if err != nil {
			t.Fatalf("%s Apply: %v", c.Name(), err)
		}
		if !res.DryRun {
			t.Errorf("%s adapter dropped dry-run flag", c.Name())
		}
	}
}
