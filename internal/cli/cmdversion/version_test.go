package cmdversion

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestVersion_Text(t *testing.T) {
	t.Parallel()
	cmd := New("9.9.9-test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "forge 9.9.9-test ") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestVersion_JSON(t *testing.T) {
	t.Parallel()
	cmd := New("9.9.9-test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v: %q", err, out.String())
	}
	if got["version"] != "9.9.9-test" {
		t.Fatalf("version = %q", got["version"])
	}
	for _, k := range []string{"go_version", "os", "arch"} {
		if got[k] == "" {
			t.Errorf("missing %s", k)
		}
	}
}
