package cmdexplain

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/verbmeta"
)

func TestExplain_ListsAllVerbs(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "explain") {
		t.Fatalf("listing missing self: %s", got)
	}
}

func TestExplain_OneVerbJSON(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"explain", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var m verbmeta.Manifest
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("not JSON: %v: %s", err, out.String())
	}
	if m.Verb != "explain" {
		t.Fatalf("unexpected manifest: %+v", m)
	}
}

func TestExplain_UnknownVerb(t *testing.T) {
	t.Parallel()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"definitely-not-a-verb"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error, got: %s", out.String())
	}
	if !strings.Contains(err.Error(), "FORGE-1400") {
		t.Fatalf("want FORGE-1400, got: %v", err)
	}
}
