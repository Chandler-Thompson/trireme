// Package config loads an instance's settings file.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// Card returns a project seat's briefing from the doctrine: the file every seat shares,
// roles/AGENTS.md, followed by the seat's own roles/<seat>/AGENTS.md.
func (inst Instance) Card(seatID string) (string, error) {
	shared, err := inst.readRole("AGENTS.md")
	if err != nil {
		return "", err
	}
	own, err := inst.readRole(filepath.Join(seatID, "AGENTS.md"))
	if err != nil {
		return "", err
	}
	return shared + "\n\n" + own, nil
}

// ProjectsBrief lists the instance's projects and where their repos are, so the XO can
// find a project's PRD and docs without searching the machine.
func (inst Instance) ProjectsBrief() string {
	var b strings.Builder
	b.WriteString("\n\n## Projects\n\n")
	if len(inst.Projects) == 0 {
		b.WriteString("This instance has no projects yet.\n")
	}
	for _, p := range inst.Projects {
		name := p.Name
		if name == "" {
			name = p.ID
		}
		fmt.Fprintf(&b, "- %s (id %q): repo at %s\n", name, p.ID, p.Repo)
	}
	return b.String()
}

// XOCard returns the XO's briefing. The XO holds no project seat, so it gets its own file
// only, without the file every project seat shares.
func (inst Instance) XOCard() (string, error) {
	return inst.readRole(filepath.Join("xo", "AGENTS.md"))
}

func (inst Instance) readRole(rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(rel, "..") {
		return "", fmt.Errorf("config: bad role path %q", rel)
	}
	raw, err := os.ReadFile(filepath.Join(inst.DoctrineDir, "roles", rel))
	if err != nil {
		return "", fmt.Errorf("config: role briefing %s: %w", rel, err)
	}
	return string(raw), nil
}
