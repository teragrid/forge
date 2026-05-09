// Package incident implements the ADR-021 incident state-machine and
// persistence model (DEV-M3-06). Incidents are stored as JSON files under
// .forge/incidents/<id>.json. The generator also writes cstate-compatible
// Markdown to content/issues/<id>.md when --render is requested.
package incident

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultDir is the project-relative directory for incident JSON files.
const DefaultDir = ".forge/incidents"

// State is an ADR-021 lifecycle state.
type State string

const (
	StateIdentified          State = "identified"
	StateInvestigating       State = "investigating"
	StateMonitoring          State = "monitoring"
	StateMitigated           State = "mitigated"
	StateFixed               State = "fixed"
	StatePostMortemPublished State = "post-mortem-published"
)

// validTransitions is the ADR-021 state machine (illegal skips are rejected).
var validTransitions = map[State][]State{
	StateIdentified:          {StateInvestigating},
	StateInvestigating:       {StateMonitoring, StateMitigated},
	StateMonitoring:          {StateInvestigating, StateMitigated},
	StateMitigated:           {StateFixed},
	StateFixed:               {StatePostMortemPublished},
	StatePostMortemPublished: {},
}

// CanTransition reports whether from → to is a legal state-machine move.
func CanTransition(from, to State) bool {
	for _, s := range validTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// Severity is the incident impact tier.
type Severity string

const (
	SeverityS0 Severity = "S0"
	SeverityS1 Severity = "S1"
	SeverityS2 Severity = "S2"
	SeverityS3 Severity = "S3"
)

// Incident is the canonical record for one incident.
type Incident struct {
	APIVersion string    `json:"api_version"`
	Kind       string    `json:"kind"`
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	State      State     `json:"state"`
	Severity   Severity  `json:"severity"`
	Systems    []string  `json:"systems"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Postmortem string    `json:"postmortem,omitempty"`
	Notes      []string  `json:"notes"`
}

// IsOpen reports whether the incident is still unresolved.
func (i *Incident) IsOpen() bool {
	return i.State != StateFixed && i.State != StatePostMortemPublished
}

// Validate returns an error if required fields are missing or invalid.
func (i *Incident) Validate() error {
	if i.ID == "" {
		return fmt.Errorf("incident: id is required")
	}
	if i.Title == "" {
		return fmt.Errorf("incident %s: title is required", i.ID)
	}
	switch i.Severity {
	case SeverityS0, SeverityS1, SeverityS2, SeverityS3:
	default:
		return fmt.Errorf("incident %s: unknown severity %q (valid: S0 S1 S2 S3)", i.ID, i.Severity)
	}
	switch i.State {
	case StateIdentified, StateInvestigating, StateMonitoring,
		StateMitigated, StateFixed, StatePostMortemPublished:
	default:
		return fmt.Errorf("incident %s: unknown state %q", i.ID, i.State)
	}
	return nil
}

// New creates a fresh Incident in StateIdentified.
func New(id, title string, sev Severity, systems []string) *Incident {
	now := time.Now().UTC()
	return &Incident{
		APIVersion: "forge.sh/v1",
		Kind:       "Incident",
		ID:         id,
		Title:      title,
		State:      StateIdentified,
		Severity:   sev,
		Systems:    systems,
		CreatedAt:  now,
		UpdatedAt:  now,
		Notes:      []string{},
	}
}

// filePath returns the canonical JSON path for id under dir.
func filePath(dir, id string) string {
	return filepath.Join(dir, strings.ToLower(id)+".json")
}

// Save writes inc to dir/<id>.json (mode 0644, parents created).
func Save(dir string, inc *Incident) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("incident: mkdir %s: %w", dir, err)
	}
	inc.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(inc, "", "  ")
	if err != nil {
		return fmt.Errorf("incident: marshal: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(filePath(dir, inc.ID), data, 0o600)
}

// Load reads an incident from dir/<id>.json.
func Load(dir, id string) (*Incident, error) {
	path := filePath(dir, id)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("incident: read %s: %w", path, err)
	}
	var inc Incident
	if err := json.Unmarshal(data, &inc); err != nil {
		return nil, fmt.Errorf("incident: parse %s: %w", path, err)
	}
	return &inc, nil
}

// LoadAll reads every *.json incident file from dir.
func LoadAll(dir string) ([]*Incident, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("incident: readdir %s: %w", dir, err)
	}
	var list []*Incident
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		inc, err := Load(dir, id)
		if err != nil {
			return nil, err
		}
		list = append(list, inc)
	}
	return list, nil
}

// Transition moves inc to newState, enforcing the ADR-021 state machine.
// Returns an error for illegal transitions.
func Transition(inc *Incident, newState State) error {
	if !CanTransition(inc.State, newState) {
		return fmt.Errorf("incident %s: illegal transition %s → %s", inc.ID, inc.State, newState)
	}
	inc.State = newState
	inc.UpdatedAt = time.Now().UTC()
	return nil
}

// RenderMarkdown produces a cstate-format Markdown document for inc.
// Used by `forge incident render --dir content/issues`.
func RenderMarkdown(inc *Incident) string {
	var sb strings.Builder
	resolved := !inc.IsOpen()
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "Title: %s\n", inc.Title)
	fmt.Fprintf(&sb, "Date: %s\n", inc.CreatedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&sb, "Resolved: %t\n", resolved)
	sb.WriteString("Informational: false\n")
	sb.WriteString("Pin: false\n")
	fmt.Fprintf(&sb, "Severity: %s\n", inc.Severity)
	sb.WriteString("Section: issue\n")
	if len(inc.Systems) > 0 {
		sb.WriteString("Systems:\n")
		for _, s := range inc.Systems {
			fmt.Fprintf(&sb, "  - %s\n", s)
		}
	}
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "\n**State:** %s\n", inc.State)
	if len(inc.Notes) > 0 {
		sb.WriteString("\n## Updates\n\n")
		for _, n := range inc.Notes {
			fmt.Fprintf(&sb, "- %s\n", n)
		}
	}
	if inc.Postmortem != "" {
		fmt.Fprintf(&sb, "\n## Post-mortem\n\nSee: %s\n", inc.Postmortem)
	}
	return sb.String()
}
