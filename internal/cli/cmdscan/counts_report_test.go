package cmdscan_test

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/teragrid/forge/internal/cli/cmdscan"
)

func TestReportFindingsCounts(_ *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(filename), "fixtures", "canonical-project")
	families := []struct {
		name string
		fn   func(string) (*cmdscan.ScanResult, error)
	}{
		{"security", cmdscan.RunSecurity},
		{"correctness", cmdscan.RunCorrectness},
		{"performance", cmdscan.RunPerformance},
		{"reliability", cmdscan.RunReliability},
		{"accessibility", cmdscan.RunAccessibility},
		{"cost", cmdscan.RunCost},
		{"compliance", cmdscan.RunCompliance},
		{"dx", cmdscan.RunDX},
	}
	for _, f := range families {
		res, err := f.fn(root)
		if err != nil {
			fmt.Printf("[%s] Error: %v\n", f.name, err)
			continue
		}
		fmt.Printf("[%s] Findings: %d\n", f.name, len(res.Findings))
	}
}
