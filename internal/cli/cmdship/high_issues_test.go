package cmdship

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teragrid/forge/internal/llmprovider"
)

// Issue #61: with only -n given (no description) the breakdown checkpoint must
// still ask the model instead of printing "run forge ship breakdown".
func TestBreakdown_NameOnlyStillGenerates(t *testing.T) {
	root := initShipRepo(t)
	dir := filepath.Join(root, ".forge", "specs", "feat-x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	calls := 0
	mock := &llmprovider.MockProvider{Fn: func(*llmprovider.Request) (*llmprovider.Response, error) {
		calls++
		return mockResponse("## T1\nDo the thing.\n"), nil
	}}
	_ = checkBreakdown(root, "", "feat-x", mockPipe(root, mock))
	if calls == 0 {
		t.Fatal("expected the model to be asked for the breakdown when only --name is given")
	}
}

// Issue #60: a checkpoint status record must not occupy an artefact's path
// and a warning record must not count as done.
func TestCheckpointDone_WarningRecordIsNotDone(t *testing.T) {
	dir := t.TempDir()
	rec := filepath.Join(dir, ".checkpoints")
	if err := os.MkdirAll(rec, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rec, "breakdown.md"),
		[]byte("# Breakdown checkpoint\n\nStatus: warning\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if checkpointDone(dir, "breakdown.md") {
		t.Fatal("warning record must not count as done")
	}
	// Legacy record written over the artefact path.
	_ = os.RemoveAll(rec)
	if err := os.WriteFile(filepath.Join(dir, "breakdown.md"),
		[]byte("# Breakdown checkpoint\n\nStatus: warning\nEvidence: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if checkpointDone(dir, "breakdown.md") {
		t.Fatal("legacy warning record must not count as done")
	}
	if err := os.WriteFile(filepath.Join(dir, "breakdown.md"), []byte("## T1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !checkpointDone(dir, "breakdown.md") {
		t.Fatal("a real breakdown.md is done")
	}
}

// Issue #58: the unverified-reference callout must not be written into the artefact.
func TestUnverifiedWarningNotInArtefact(t *testing.T) {
	out := appendUnverifiedPathsWarning(t.TempDir(), "see `nope/x.ts`")
	if strings.Contains(out, "Unverified") {
		t.Fatal("callout leaked into artefact")
	}
}
