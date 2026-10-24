// Package inbox holds items addressed to a seat until its next turn: directives from
// the user through the XO, consults from other seats, and answers to its blockers.
// Every item is also a journal event, so nothing said here is off the record.
package inbox

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Chandler-Thompson/trireme/internal/journal"
)

// Item kinds.
const (
	KindDirective = "directive"
	KindConsult   = "consult"
	KindAnswer    = "answer"
	KindKick      = "kick"
)

// Item is one thing waiting for a seat.
type Item struct {
	ID        int64     `json:"id"`
	ProjectID string    `json:"project_id"`
	ToSeat    string    `json:"to_seat"`
	FromSeat  string    `json:"from_seat"`
	Kind      string    `json:"kind"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// Inbox stores items per seat.
type Inbox struct {
	db      *sql.DB
	journal *journal.Journal
	now     func() time.Time
}

// New returns an Inbox.
func New(db *sql.DB, j *journal.Journal, now func() time.Time) *Inbox {
	return &Inbox{db: db, journal: j, now: now}
}

var eventFor = map[string]string{
	KindDirective: "directive.posted",
	KindConsult:   "consult.opened",
	KindAnswer:    "consult.answered",
	KindKick:      "seat.kicked",
}

// Post adds an item and journals it. The item is stored only if the journal accepts it,
// so the inbox never holds anything the record does not.
func (in *Inbox) Post(it Item) (Item, error) {
	event, ok := eventFor[it.Kind]
	if !ok {
		return Item{}, fmt.Errorf("inbox: unknown kind %q", it.Kind)
	}
	if it.ProjectID == "" || it.ToSeat == "" || it.FromSeat == "" {
		return Item{}, fmt.Errorf("inbox: project, to and from are required")
	}
	if _, err := in.journal.Append(journal.Event{ProjectID: it.ProjectID, Seat: it.FromSeat, Type: event,
		Payload: map[string]any{"to": it.ToSeat, "body": it.Body}}); err != nil {
		return Item{}, fmt.Errorf("inbox: journal: %w", err)
	}
	it.CreatedAt = in.now().UTC()
	res, err := in.db.Exec(`INSERT INTO inbox (project_id, to_seat, from_seat, kind, body, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, it.ProjectID, it.ToSeat, it.FromSeat, it.Kind, it.Body,
		it.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return Item{}, fmt.Errorf("inbox: insert: %w", err)
	}
	if it.ID, err = res.LastInsertId(); err != nil {
		return Item{}, fmt.Errorf("inbox: read id: %w", err)
	}
	return it, nil
}

// Pending returns undelivered items for one seat, oldest first.
func (in *Inbox) Pending(projectID, seat string) ([]Item, error) {
	rows, err := in.db.Query(`SELECT id, project_id, to_seat, from_seat, kind, body, created_at FROM inbox
		WHERE project_id = ? AND to_seat = ? AND delivered_at IS NULL ORDER BY id`, projectID, seat)
	if err != nil {
		return nil, fmt.Errorf("inbox: query: %w", err)
	}
	defer func() { _ = rows.Close() }() // why: rows.Err() reports iteration failures
	var out []Item
	for rows.Next() {
		var it Item
		var created string
		if err := rows.Scan(&it.ID, &it.ProjectID, &it.ToSeat, &it.FromSeat, &it.Kind, &it.Body, &created); err != nil {
			return nil, fmt.Errorf("inbox: scan: %w", err)
		}
		if it.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
			return nil, fmt.Errorf("inbox: parse created_at of %d: %w", it.ID, err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// MarkDelivered records that items reached their seat in a turn.
func (in *Inbox) MarkDelivered(items []Item) error {
	at := in.now().UTC().Format(time.RFC3339Nano)
	for _, it := range items {
		if _, err := in.db.Exec(`UPDATE inbox SET delivered_at = ? WHERE id = ?`, at, it.ID); err != nil {
			return fmt.Errorf("inbox: mark %d delivered: %w", it.ID, err)
		}
	}
	return nil
}
