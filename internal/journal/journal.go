// Package journal is the instance's single event record (doctrine module "journal").
// Only machinery writes to it; a seat never reports on itself here.
package journal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Event is one journal record, matching the doctrine's envelope.
type Event struct {
	ID              int64          `json:"id"`
	TS              time.Time      `json:"ts"`
	ProjectID       string         `json:"project_id"`
	DoctrineVersion string         `json:"doctrine_version"`
	Seat            string         `json:"seat"`
	TicketID        string         `json:"ticket_id,omitempty"`
	Type            string         `json:"type"`
	Payload         map[string]any `json:"payload"`
}

// Journal appends and reads events.
type Journal struct {
	db  *sql.DB
	now func() time.Time
}

// New returns a Journal over an opened store database.
func New(db *sql.DB, now func() time.Time) *Journal {
	return &Journal{db: db, now: now}
}

// Append records an event, stamping its time, and returns it with its id.
func (j *Journal) Append(e Event) (Event, error) {
	if e.ProjectID == "" || e.Seat == "" || e.Type == "" {
		return Event{}, fmt.Errorf("journal: project, seat and type are required")
	}
	if e.Payload == nil {
		e.Payload = map[string]any{}
	}
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return Event{}, fmt.Errorf("journal: encode payload: %w", err)
	}
	e.TS = j.now().UTC()
	res, err := j.db.Exec(
		`INSERT INTO events (ts, project_id, doctrine_version, seat, ticket_id, type, payload)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.TS.Format(time.RFC3339Nano), e.ProjectID, e.DoctrineVersion, e.Seat,
		nullable(e.TicketID), e.Type, string(payload))
	if err != nil {
		return Event{}, fmt.Errorf("journal: insert: %w", err)
	}
	if e.ID, err = res.LastInsertId(); err != nil {
		return Event{}, fmt.Errorf("journal: read id: %w", err)
	}
	return e, nil
}

// Since returns up to limit events with an id greater than afterID, oldest first.
// An empty projectID returns events for every project.
func (j *Journal) Since(projectID string, afterID int64, limit int) ([]Event, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("journal: limit must be positive, got %d", limit)
	}
	rows, err := j.db.Query(
		`SELECT id, ts, project_id, doctrine_version, seat, COALESCE(ticket_id, ''), type, payload
		 FROM events WHERE id > ? AND (? = '' OR project_id = ?) ORDER BY id LIMIT ?`,
		afterID, projectID, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("journal: query: %w", err)
	}
	defer closeRows(rows)
	var out []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func scanEvent(rows *sql.Rows) (Event, error) {
	var e Event
	var ts, payload string
	if err := rows.Scan(&e.ID, &ts, &e.ProjectID, &e.DoctrineVersion, &e.Seat,
		&e.TicketID, &e.Type, &payload); err != nil {
		return Event{}, fmt.Errorf("journal: scan: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return Event{}, fmt.Errorf("journal: parse ts %q: %w", ts, err)
	}
	e.TS = parsed
	if err := json.Unmarshal([]byte(payload), &e.Payload); err != nil {
		return Event{}, fmt.Errorf("journal: decode payload of event %d: %w", e.ID, err)
	}
	return e, nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// closeRows closes a result set; a close error after a full read loses nothing.
func closeRows(rows *sql.Rows) {
	_ = rows.Close() // why: rows.Err() already reports iteration failures
}

// Spent sums the API-equivalent cost of every model call recorded for a project.
func (j *Journal) Spent(projectID string) (float64, error) {
	var total sql.NullFloat64
	err := j.db.QueryRow(
		`SELECT SUM(CAST(json_extract(payload, '$.cost_usd') AS REAL))
		 FROM events WHERE project_id = ? AND type = 'model.call'`, projectID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("journal: sum spend: %w", err)
	}
	return total.Float64, nil
}
