// Package config loads an instance's settings file.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Seat is one staffed seat in a project.
type Seat struct {
	ID             string   `json:"id"`
	Model          string   `json:"model"`
	AllowedTools   []string `json:"allowed_tools"`
	PermissionMode string   `json:"permission_mode"`
}

// Project is one project and its fleet.
type Project struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Repo      string  `json:"repo"`
	CapUSD    float64 `json:"cap_usd"`
	CompactAt int64   `json:"compact_at"`
	Seats     []Seat  `json:"seats"`
}

// XO configures the XO seat. Agent names a harness-side agent definition, which is how
// a user plugs their own persistent agent into the seat.
type XO struct {
	Agent          string   `json:"agent"`
	Model          string   `json:"model"`
	WorkDir        string   `json:"workdir"`
	AllowedTools   []string `json:"allowed_tools"`
	PermissionMode string   `json:"permission_mode"`
}

// Instance is the whole settings file.
type Instance struct {
	Listen          string `json:"listen"`
	DataDir         string `json:"data_dir"`
	BodyLabel       string `json:"body_label"`
	ClaudeBinary    string `json:"claude_binary"`
	DoctrineDir     string `json:"doctrine_dir"`
	DoctrineVersion string `json:"doctrine_version"`
	// TokenFile holds the API token. It is required whenever Listen is not loopback.
	TokenFile string `json:"token_file"`
	// NotifyCommand, when set, runs once per new blocker with the blocker as JSON on
	// standard input. It is how an instance plugs in its own notifier.
	NotifyCommand []string  `json:"notify_command"`
	XO            XO        `json:"xo"`
	Projects      []Project `json:"projects"`
}

// Load reads and validates a settings file.
func Load(path string) (Instance, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Instance{}, fmt.Errorf("config: read %s: %w", path, err)
	}
	var inst Instance
	if err := json.Unmarshal(raw, &inst); err != nil {
		return Instance{}, fmt.Errorf("config: parse %s: %w", path, err)
	}
	applyDefaults(&inst)
	return inst, validate(inst)
}

func applyDefaults(inst *Instance) {
	if inst.Listen == "" {
		inst.Listen = "127.0.0.1:8740"
	}
	if inst.ClaudeBinary == "" {
		inst.ClaudeBinary = "claude"
	}
	if inst.BodyLabel == "" {
		inst.BodyLabel = "this machine"
	}
	for i := range inst.Projects {
		if inst.Projects[i].CompactAt == 0 {
			inst.Projects[i].CompactAt = 200_000
		}
	}
}

func validate(inst Instance) error {
	if inst.DataDir == "" || inst.DoctrineDir == "" {
		return fmt.Errorf("config: data_dir and doctrine_dir are required")
	}
	seen := map[string]bool{}
	for _, p := range inst.Projects {
		if p.ID == "" || p.Repo == "" || len(p.Seats) == 0 {
			return fmt.Errorf("config: project %q needs an id, a repo and at least one seat", p.ID)
		}
		if seen[p.ID] {
			return fmt.Errorf("config: duplicate project %q", p.ID)
		}
		seen[p.ID] = true
		if p.CapUSD <= 0 {
			return fmt.Errorf("config: project %q has no cap; a fleet does not start without one", p.ID)
		}
	}
	return nil
}

// Card returns a seat's role card from the doctrine.
func (inst Instance) Card(seatID string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(inst.DoctrineDir, "roles", seatID+".md"))
	if err != nil {
		return "", fmt.Errorf("config: role card for %s: %w", seatID, err)
	}
	return string(raw), nil
}
