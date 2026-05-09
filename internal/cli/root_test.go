package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestRootCommand_Version exercises the version flag end-to-end so that
// DEV-M0-01 TC-01-06 (data-accuracy: --version surfaces the injected build
// version) has a regression anchor at the unit-test layer.
func TestRootCommand_Version(t *testing.T) {
	t.Parallel()

	const want = "1.2.3-test"
	cmd := NewRootCommand(want)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "forge "+want {
		t.Fatalf("version output = %q, want %q", got, "forge "+want)
	}
}

// TestRootCommand_Help guards that --help exits cleanly and prints the binary
// name. Acts as the false-positive guard in the TEST-01 9-point checklist.
func TestRootCommand_Help(t *testing.T) {
	t.Parallel()

	cmd := NewRootCommand("0.0.0-dev")

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() --help returned error: %v", err)
	}

	if !strings.Contains(out.String(), "forge") {
		t.Fatalf("help output missing binary name; got: %q", out.String())
	}
}
