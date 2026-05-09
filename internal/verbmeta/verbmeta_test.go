package verbmeta

import "testing"

func TestRegisterAndLookup(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	Register(Manifest{Verb: "demo", Summary: "demo verb"})
	m, ok := Lookup("demo")
	if !ok {
		t.Fatal("Lookup missed registered verb")
	}
	if m.Inputs == nil || m.Outputs == nil || m.SideEffects == nil {
		t.Fatal("nil slices must be normalised to empty for stable JSON")
	}
}

func TestRegister_DuplicatePanics(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	Register(Manifest{Verb: "dup"})
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate verb")
		}
	}()
	Register(Manifest{Verb: "dup"})
}

func TestRegister_EmptyVerbPanics(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on empty verb")
		}
	}()
	Register(Manifest{Verb: ""})
}

func TestAll_Sorted(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	Register(Manifest{Verb: "zeta"})
	Register(Manifest{Verb: "alpha"})
	Register(Manifest{Verb: "mu"})
	out := All()
	if len(out) != 3 || out[0].Verb != "alpha" || out[1].Verb != "mu" || out[2].Verb != "zeta" {
		t.Fatalf("All() not sorted: %+v", out)
	}
}
