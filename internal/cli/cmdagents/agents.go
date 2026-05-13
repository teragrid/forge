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

// Package cmdagents implements `forge agents` (M2-31).
//
// The agents namespace manages long-running background AI agent processes
// that Forge can supervise: start, stop, list, and inspect status.
//
// Agent state is stored in .forge/agents.json. In M2 the "start" command
// records the intent and PID; in M3 it wires to the multi-agent runtime.
package cmdagents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/teragrid/forge/internal/errcode"
	"github.com/teragrid/forge/internal/verbmeta"
)

// Reserved error codes (range 5400..5499).
var (
	ErrAgentsFailed = errcode.Register(errcode.Code(5400), "agents operation failed")
)

const agentsStatePath = ".forge/agents.json"

// AgentEntry records a registered agent process.
type AgentEntry struct {
	Name      string `json:"name"`
	Role      string `json:"role,omitempty"` // e.g. "scanner", "reviewer", "coder"
	StartedAt string `json:"started_at"`
	PID       int    `json:"pid,omitempty"`
	Status    string `json:"status"` // "running" | "stopped" | "unknown"
}

func init() {
	verbmeta.Register(verbmeta.Manifest{
		Verb:    "agents",
		Summary: "Start, stop, and list background AI agent processes (M2).",
		Inputs: []string{
			"start <name>  — register and start a named agent",
			"stop  <name>  — stop a running agent",
			"list          — list all registered agents",
			"inspect <name>— show agent details",
			"--role <role>",
			"--root <path>",
			"--json",
		},
		Outputs:      []string{"stdout: agent operation result"},
		SideEffects:  []string{"start/stop: mutate .forge/agents.json"},
		GatesTouched: []string{"§17 multi-agent runtime"},
		ErrorCodes:   []errcode.Code{ErrAgentsFailed},
	})
}

// New returns the cobra command for `forge agents`.
func New() *cobra.Command {
	var (
		root    string
		jsonOut bool
	)
	cmd := &cobra.Command{
		Use:   "agents <start|stop|list|inspect>",
		Short: "Manage background AI agent processes (M2).",
		Long: "forge agents manages long-running AI agent processes.\n\n" +
			"State is persisted in .forge/agents.json.\n\n" +
			"Examples:\n" +
			"  forge agents start reviewer --role reviewer\n" +
			"  forge agents list\n" +
			"  forge agents stop reviewer",
	}
	cmd.PersistentFlags().StringVar(&root, "root", "", "Project root (default: cwd)")
	cmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Emit JSON output")

	// start
	startCmd := &cobra.Command{
		Use:   "start <name>",
		Short: "Register and start a named agent.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			role, _ := cmd.Flags().GetString("role")
			r, _ := resolveAgentsRoot(root)
			entry := AgentEntry{
				Name:      args[0],
				Role:      role,
				StartedAt: time.Now().UTC().Format(time.RFC3339),
				Status:    "running",
			}
			agents, _ := loadAgents(r)
			agents = upsertAgent(agents, entry)
			if err := saveAgents(r, agents); err != nil {
				return errcode.New(ErrAgentsFailed, "save agents state", err)
			}
			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(entry)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "agents start: registered agent %q (role=%s)\n", entry.Name, entry.Role)
			return nil
		},
	}
	startCmd.Flags().String("role", "", "Agent role (scanner|reviewer|coder|planner)")

	// stop
	stopCmd := &cobra.Command{
		Use:   "stop <name>",
		Short: "Stop a running agent.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, _ := resolveAgentsRoot(root)
			agents, _ := loadAgents(r)
			found := false
			for i := range agents {
				if agents[i].Name == args[0] {
					agents[i].Status = "stopped"
					found = true
				}
			}
			if !found {
				return errcode.Newf(ErrAgentsFailed, nil, "agent %q not found", args[0])
			}
			if err := saveAgents(r, agents); err != nil {
				return errcode.New(ErrAgentsFailed, "save agents state", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "agents stop: agent %q stopped\n", args[0])
			return nil
		},
	}

	// list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all registered agents.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, _ := resolveAgentsRoot(root)
			agents, _ := loadAgents(r)
			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(agents)
			}
			if len(agents) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "agents list: no agents registered")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-12s %-10s %s\n", "NAME", "ROLE", "STATUS", "STARTED")
			for _, a := range agents {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-12s %-10s %s\n", a.Name, a.Role, a.Status, a.StartedAt)
			}
			return nil
		},
	}

	// inspect
	inspectCmd := &cobra.Command{
		Use:   "inspect <name>",
		Short: "Show details for a specific agent.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, _ := resolveAgentsRoot(root)
			agents, _ := loadAgents(r)
			for _, a := range agents {
				if a.Name == args[0] {
					if jsonOut {
						return json.NewEncoder(cmd.OutOrStdout()).Encode(a)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "name:     %s\nrole:     %s\nstatus:   %s\nstarted:  %s\npid:      %d\n",
						a.Name, a.Role, a.Status, a.StartedAt, a.PID)
					return nil
				}
			}
			return errcode.Newf(ErrAgentsFailed, nil, "agent %q not found", args[0])
		},
	}

	cmd.AddCommand(startCmd, stopCmd, listCmd, inspectCmd)
	return cmd
}

func resolveAgentsRoot(root string) (string, error) {
	if root != "" {
		return root, nil
	}
	return os.Getwd()
}

func loadAgents(root string) ([]AgentEntry, error) {
	data, err := os.ReadFile(filepath.Join(root, agentsStatePath))
	if err != nil {
		return nil, nil
	}
	var agents []AgentEntry
	return agents, json.Unmarshal(data, &agents)
}

func saveAgents(root string, agents []AgentEntry) error {
	path := filepath.Join(root, agentsStatePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(agents, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func upsertAgent(agents []AgentEntry, e AgentEntry) []AgentEntry {
	for i := range agents {
		if agents[i].Name == e.Name {
			agents[i] = e
			return agents
		}
	}
	return append(agents, e)
}
