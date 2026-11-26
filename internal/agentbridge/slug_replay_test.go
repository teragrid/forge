package agentbridge

import (
	"errors"
	"testing"
)

// A response recorded for one feature must never be replayed by position into
// another (issue #56).
func TestLookup_OrdinalFallbackIgnoresOtherSlug(t *testing.T) {
	root := t.TempDir()
	b, err := Open(root, "")
	if err != nil {
		t.Fatal(err)
	}
	b.SetFeature("A", "a")
	if _, err := b.Lookup("op", "spec", "", "sys", "prompt for a", 10); !errors.Is(err, ErrTurnRequired) {
		t.Fatalf("want turn required, got %v", err)
	}
	if _, err := b.Fulfil("answer for a"); err != nil {
		t.Fatal(err)
	}

	// Same operation, same position, different feature slug and prompt.
	b2, err := Open(root, "")
	if err != nil {
		t.Fatal(err)
	}
	b2.session.Slug = "b" // simulate a session that no longer resets on switch
	if got, err := b2.Lookup("op", "spec", "", "sys", "prompt for b", 10); err == nil {
		t.Fatalf("replayed another feature's answer: %q", got)
	}
}

func TestHash_IgnoresGeneratedTimestamp(t *testing.T) {
	a := Hash("op", "s", "x\nGenerated: 2026-01-01T00:00:00Z\ny")
	b := Hash("op", "s", "x\nGenerated: 2026-02-02T10:11:12Z\ny")
	if a != b {
		t.Fatal("volatile Generated line must not change the hash")
	}
}
