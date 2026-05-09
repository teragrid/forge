// Package codemod implements `forge upgrade` (M2 codemod runner).
// A codemod is a deterministic, idempotent transformation of project files.
// Built-in codemods cover gitignore-marker drift and gitleaks-baseline drift.
package codemod

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Codemod is the interface a transformation must satisfy.
type Codemod interface {
	Name() string
	Description() string
	Apply(root string, dryRun bool) (Report, error)
}

// Report is the per-codemod outcome.
type Report struct {
	Codemod string   `json:"codemod"`
	Files   []string `json:"files"` // files touched (or would-touch in dryRun)
	Changed int      `json:"changed"`
	DryRun  bool     `json:"dry_run"`
	Detail  string   `json:"detail,omitempty"`
}

// Registry stores in-tree codemods.
type Registry struct {
	mu sync.RWMutex
	m  map[string]Codemod
}

var defaultRegistry = NewRegistry()

func Default() *Registry { return defaultRegistry }
func NewRegistry() *Registry {
	return &Registry{m: map[string]Codemod{}}
}

func (r *Registry) Register(c Codemod) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.m[c.Name()]; ok {
		panic("codemod: duplicate " + c.Name())
	}
	r.m[c.Name()] = c
}

func (r *Registry) Lookup(name string) (Codemod, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.m[name]
	return c, ok
}

func (r *Registry) All() []Codemod {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Codemod, 0, len(r.m))
	for _, c := range r.m {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// Reset (test-only).
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m = map[string]Codemod{}
}

// ----- built-in codemods -----

func init() {
	Default().Register(&gitignoreMarker{})
	Default().Register(&gitleaksBaseline{})
}

// gitignoreMarker ensures .gitignore has a forge marker block.
type gitignoreMarker struct{}

func (gitignoreMarker) Name() string { return "gitignore-marker" }
func (gitignoreMarker) Description() string {
	return "Insert/refresh `# forge:gitignore:start` marker block in .gitignore."
}

const markerStart = "# forge:gitignore:start"
const markerEnd = "# forge:gitignore:end"

var markerBlockRE = regexp.MustCompile(`(?ms)^# forge:gitignore:start.*?^# forge:gitignore:end\s*`)

const defaultMarkerBody = `# forge:gitignore:start
# Managed by forge — do not edit manually. Run "forge upgrade gitignore-marker" to refresh.
.forge/scratch/
.forge/cache/
*.tmp
*.bak
__pycache__/
# forge:gitignore:end
`

func (g gitignoreMarker) Apply(root string, dryRun bool) (Report, error) {
	path := filepath.Join(root, ".gitignore")
	rep := Report{Codemod: g.Name(), DryRun: dryRun}
	body, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return rep, fmt.Errorf("read .gitignore: %w", err)
	}
	current := string(body)
	var next string
	if markerBlockRE.MatchString(current) {
		next = markerBlockRE.ReplaceAllString(current, defaultMarkerBody)
	} else {
		if current != "" && !strings.HasSuffix(current, "\n") {
			current += "\n"
		}
		next = current + defaultMarkerBody
	}
	if next == current {
		rep.Detail = "no change"
		return rep, nil
	}
	rep.Files = []string{".gitignore"}
	rep.Changed = 1
	if !dryRun {
		if err := os.WriteFile(path, []byte(next), 0o600); err != nil {
			return rep, fmt.Errorf("write .gitignore: %w", err)
		}
	}
	return rep, nil
}

// gitleaksBaseline ensures .gitleaks.toml exists with baseline rules.
type gitleaksBaseline struct{}

func (gitleaksBaseline) Name() string { return "gitleaks-baseline" }
func (gitleaksBaseline) Description() string {
	return "Create .gitleaks.toml with the forge baseline rules if missing."
}

const defaultGitleaksBody = `# Managed by forge — baseline rules. Add custom rules below.
title = "forge baseline"

[[rules]]
id = "generic-api-key"
description = "Generic API key"
regex = '''(?i)(api[_-]?key|apikey)[\s:=]+["']?[A-Za-z0-9_\-]{20,}["']?'''

[[rules]]
id = "private-key-block"
description = "PEM private key"
regex = '''-----BEGIN [A-Z ]*PRIVATE KEY-----'''

[[rules]]
id = "openai-key"
description = "OpenAI key"
regex = '''sk-[A-Za-z0-9]{20,}'''

[[rules]]
id = "aws-access-key"
description = "AWS access key"
regex = '''AKIA[0-9A-Z]{16}'''
`

func (g gitleaksBaseline) Apply(root string, dryRun bool) (Report, error) {
	path := filepath.Join(root, ".gitleaks.toml")
	rep := Report{Codemod: g.Name(), DryRun: dryRun}
	if _, err := os.Stat(path); err == nil {
		rep.Detail = "already present"
		return rep, nil
	}
	rep.Files = []string{".gitleaks.toml"}
	rep.Changed = 1
	if !dryRun {
		if err := os.WriteFile(path, []byte(defaultGitleaksBody), 0o600); err != nil {
			return rep, fmt.Errorf("write .gitleaks.toml: %w", err)
		}
	}
	return rep, nil
}
