// Package fleet holds each project's fleet state (doctrine module "budgets": pause,
// stop, archive).
package fleet

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/Chandler-Thompson/trireme/internal/journal"
)

// States a fleet can be in.
const (
	Running  = "running"
	Paused   = "paused"
	Stopped  = "stopped"
	Archived = "archived"
)

// Reasons a fleet pauses.
const (
	ReasonUser        = "user"
	ReasonUsageWindow = "usage_window"
	ReasonProjectCap  = "project_cap"
)

// ErrUnknown is returned for a project that has no fleet.
var ErrUnknown = errors.New("fleet: no fleet for project")

// State is a fleet's current state and why it is in it.
type State struct {
	ProjectID string `json:"project_id"`
	State     string `json:"state"`
	Reason    string `json:"reason"`
}

// Registry tracks every fleet.
type Registry struct {
	db      *sql.DB
	journal *journal.Journal
}

// New returns a Registry.
func New(db *sql.DB, j *journal.Journal) *Registry {
	return &Registry{db: db, journal: j}
}

// Get returns a fleet's state.
func (r *Registry) Get(projectID string) (State, error) {
	s := State{ProjectID: projectID}
	err := r.db.QueryRow(`SELECT state, reason FROM fleets WHERE project_id = ?`, projectID).Scan(&s.State, &s.Reason)
	if errors.Is(err, sql.ErrNoRows) {
		return State{}, ErrUnknown
	}
	if err != nil {
		return State{}, fmt.Errorf("fleet: get %s: %w", projectID, err)
	}
	return s, nil
}

// List returns every fleet that is not archived.
func (r *Registry) List() ([]State, error) {
	rows, err := r.db.Query(`SELECT project_id, state, reason FROM fleets WHERE state != ? ORDER BY project_id`, Archived)
	if err != nil {
		return nil, fmt.Errorf("fleet: list: %w", err)
	}
	defer func() { _ = rows.Close() }() // why: rows.Err() reports iteration failures
	var out []State
	for rows.Next() {
		var s State
		if err := rows.Scan(&s.ProjectID, &s.State, &s.Reason); err != nil {
			return nil, fmt.Errorf("fleet: scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Start starts a new fleet, or restarts a stopped one.
func (r *Registry) Start(projectID string) error {
	current, err := r.Get(projectID)
	if err != nil && !errors.Is(err, ErrUnknown) {
		return err
	}
	if err == nil && current.State != Stopped {
		return fmt.Errorf("fleet: %s is %s, not stopped", projectID, current.State)
	}
	return r.set(projectID, Running, "", "fleet.started")
}

// Pause pauses a running fleet. Pausing a paused fleet is a no-op, so concurrent seats
// hitting the same limit produce one pause.
func (r *Registry) Pause(projectID, reason string) (changed bool, err error) {
	current, err := r.Get(projectID)
	if err != nil {
		return false, err
	}
	if current.State == Paused {
		return false, nil
	}
	if current.State != Running {
		return false, fmt.Errorf("fleet: cannot pause %s from %s", projectID, current.State)
	}
	return true, r.set(projectID, Paused, reason, "fleet.paused")
}

// Resume resumes a paused fleet. Only the user resumes a fleet.
func (r *Registry) Resume(projectID string) error {
	return r.transition(projectID, Paused, Running, "fleet.resumed")
}

// Stop stops a running or paused fleet. Its information is kept.
func (r *Registry) Stop(projectID string) error {
	current, err := r.Get(projectID)
	if err != nil {
		return err
	}
	if current.State != Running && current.State != Paused {
		return fmt.Errorf("fleet: cannot stop %s from %s", projectID, current.State)
	}
	return r.set(projectID, Stopped, "", "fleet.stopped")
}

// Archive hides a stopped fleet from every view. Its data is kept.
func (r *Registry) Archive(projectID string) error {
	return r.transition(projectID, Stopped, Archived, "fleet.archived")
}

func (r *Registry) transition(projectID, from, to, event string) error {
	current, err := r.Get(projectID)
	if err != nil {
		return err
	}
	if current.State != from {
		return fmt.Errorf("fleet: %s is %s, not %s", projectID, current.State, from)
	}
	return r.set(projectID, to, "", event)
}

func (r *Registry) set(projectID, state, reason, event string) error {
	if _, err := r.db.Exec(`INSERT INTO fleets (project_id, state, reason) VALUES (?, ?, ?)
		ON CONFLICT(project_id) DO UPDATE SET state = excluded.state, reason = excluded.reason`,
		projectID, state, reason); err != nil {
		return fmt.Errorf("fleet: set %s %s: %w", projectID, state, err)
	}
	payload := map[string]any{}
	if reason != "" {
		payload["reason"] = reason
	}
	if _, err := r.journal.Append(journal.Event{ProjectID: projectID, Seat: "system", Type: event, Payload: payload}); err != nil {
		return fmt.Errorf("fleet: journal %s: %w", event, err)
	}
	return nil
}
