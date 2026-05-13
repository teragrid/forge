// Copyright 2024 The Forge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package agents provides the multi-agent runtime foundation for Forge (M3-28).
//
// The agents runtime manages the lifecycle of long-running AI agent processes:
//
//   - Starting agents (forge agents start <name> -- <command>)
//   - Stopping agents (forge agents stop <name>)
//   - Listing agents (forge agents list)
//   - Inspecting agent status (forge agents inspect <name>)
//
// Agent records are persisted to .forge/agents.json so that state survives
// CLI invocations. Each agent is a subprocess managed via os/exec.
//
// Error code range: 6000-6099 (reserved).
//
// ADR references: ADR-002 (plugin runtime), ADR-006 (telemetry).
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// Status represents the lifecycle state of an agent.
type Status string

const (
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusStopped  Status = "stopped"
	StatusFailed   Status = "failed"
)

// Agent describes a managed agent process.
type Agent struct {
	Name      string    `json:"name"`
	Role      string    `json:"role,omitempty"`
	Command   []string  `json:"command"`
	PID       int       `json:"pid,omitempty"`
	Status    Status    `json:"status"`
	StartedAt time.Time `json:"started_at,omitempty"`
	StoppedAt time.Time `json:"stopped_at,omitempty"`
	ExitCode  int       `json:"exit_code,omitempty"`
}

// Runtime manages agent processes for a project root.
type Runtime struct {
	mu   sync.Mutex
	root string
	// live tracks running cmd handles keyed by agent name.
	live map[string]*exec.Cmd
}

// NewRuntime returns a Runtime rooted at root.
func NewRuntime(root string) *Runtime {
	return &Runtime{root: root, live: map[string]*exec.Cmd{}}
}

// storePath returns the path to .forge/agents.json.
func (r *Runtime) storePath() string {
	return filepath.Join(r.root, ".forge", "agents.json")
}

// load reads all persisted agents.
func (r *Runtime) load() ([]Agent, error) {
	data, err := os.ReadFile(r.storePath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var agents []Agent
	return agents, json.Unmarshal(data, &agents)
}

// save persists the agent list.
func (r *Runtime) save(agents []Agent) error {
	dir := filepath.Dir(r.storePath())
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(agents, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.storePath(), data, 0o644)
}

// upsert replaces the record with the same Name or appends a new one.
func upsert(agents []Agent, a Agent) []Agent {
	for i, existing := range agents {
		if existing.Name == a.Name {
			agents[i] = a
			return agents
		}
	}
	return append(agents, a)
}

// Start launches a new agent subprocess.
func (r *Runtime) Start(ctx context.Context, name, role string, command []string) (*Agent, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("agents: command must not be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	agents, err := r.load()
	if err != nil {
		return nil, fmt.Errorf("agents: load: %w", err)
	}
	for _, a := range agents {
		if a.Name == name && a.Status == StatusRunning {
			return nil, fmt.Errorf("agents: agent %q is already running (pid %d)", name, a.PID)
		}
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...) //nolint:gosec // command provided by user
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("agents: start %q: %w", name, err)
	}

	a := Agent{
		Name:      name,
		Role:      role,
		Command:   command,
		PID:       cmd.Process.Pid,
		Status:    StatusRunning,
		StartedAt: time.Now().UTC(),
	}
	r.live[name] = cmd
	agents = upsert(agents, a)
	if err := r.save(agents); err != nil {
		return nil, err
	}
	return &a, nil
}

// Stop signals the named agent to stop.
func (r *Runtime) Stop(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agents, err := r.load()
	if err != nil {
		return fmt.Errorf("agents: load: %w", err)
	}
	var target *Agent
	for i := range agents {
		if agents[i].Name == name {
			target = &agents[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("agents: agent %q not found", name)
	}
	if target.Status != StatusRunning {
		return fmt.Errorf("agents: agent %q is not running (status: %s)", name, target.Status)
	}

	if cmd, ok := r.live[name]; ok {
		if err := cmd.Process.Kill(); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("agents: stop %q: %w", name, err)
		}
		delete(r.live, name)
	} else {
		// Process started in a previous CLI invocation; kill by PID.
		if target.PID > 0 {
			proc, err := os.FindProcess(target.PID)
			if err == nil {
				_ = proc.Kill()
			}
		}
	}

	target.Status = StatusStopped
	target.StoppedAt = time.Now().UTC()
	agents = upsert(agents, *target)
	return r.save(agents)
}

// List returns all known agents.
func (r *Runtime) List() ([]Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.load()
}

// Inspect returns the agent with the given name, refreshing its live status.
func (r *Runtime) Inspect(name string) (*Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	agents, err := r.load()
	if err != nil {
		return nil, err
	}
	for _, a := range agents {
		a := a
		if a.Name == name {
			// Probe liveness: if we have a live handle, check if process exited.
			if cmd, ok := r.live[name]; ok && a.Status == StatusRunning {
				if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
					a.Status = StatusStopped
					a.ExitCode = cmd.ProcessState.ExitCode()
					a.StoppedAt = time.Now().UTC()
					agents = upsert(agents, a)
					_ = r.save(agents)
				}
			}
			return &a, nil
		}
	}
	return nil, fmt.Errorf("agents: agent %q not found", name)
}
