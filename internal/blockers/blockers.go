// Package blockers is the queue of decisions only the user can make
// (doctrine module "communication", the blockers queue).
package blockers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/Chandler-Thompson/trireme/internal/journal"
)

// Kinds the application acts on when a blocker is answered. Any other kind is free text.
const (
	KindFleetPaused = "fleet_paused"
	KindCapReached  = "cap_reached"
	KindGeneral     = "general"
)

// Choices the application acts on.
const (
	ChoiceResume   = "Resume"
	ChoiceRaiseCap = "Raise cap and resume"
)

// ErrAnswered is returned when a blocker that already has an answer is answered again.
var ErrAnswered = errors.New("blockers: already answered")

// Blocker is one decision waiting on the user.
type Blocker struct {
	ID         int64      `json:"id"`
	ProjectID  string     `json:"project_id"`
	Seat       string     `json:"seat"`
	Kind       string     `json:"kind"`
	Summary    string     `json:"summary"`
	Detail     string     `json:"detail"`
	Options    []string   `json:"options"`
	OpenedAt   time.Time  `json:"opened_at"`
	Choice     string     `json:"choice,omitempty"`
	Text       string     `json:"text,omitempty"`
	AnsweredAt *time.Time `json:"answered_at,omitempty"`
}

// Notifier is the hook an instance plugs a notifier into, for example a phone push.
type Notifier interface {
	Notify(b Blocker) error
}

// Queue stores blockers and records each change in the journal.
type Queue struct {
	db       *sql.DB
	journal  *journal.Journal
	notifier Notifier
	now      func() time.Time
	log      *slog.Logger
}

// New returns a Queue. notifier may be nil.
func New(db *sql.DB, j *journal.Journal, notifier Notifier, now func() time.Time, log *slog.Logger) *Queue {
	return &Queue{db: db, journal: j, notifier: notifier, now: now, log: log}
}

// Open adds a blocker, journals it and notifies. A notify failure is journaled, not returned,
// because the blocker itself is safely recorded.
func (q *Queue) Open(b Blocker) (Blocker, error) {
	if b.ProjectID == "" || b.Seat == "" || b.Summary == "" {
		return Blocker{}, fmt.Errorf("blockers: project, seat and summary are required")
	}
	if b.Kind == "" {
		b.Kind = KindGeneral
	}
	options, err := json.Marshal(nonNil(b.Options))
	if err != nil {
		return Blocker{}, fmt.Errorf("blockers: encode options: %w", err)
	}
	b.OpenedAt = q.now().UTC()
	res, err := q.db.Exec(`INSERT INTO blockers (project_id, seat, kind, summary, detail, options, opened_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, b.ProjectID, b.Seat, b.Kind, b.Summary, b.Detail,
		string(options), b.OpenedAt.Format(time.RFC3339Nano))
	if err != nil {
		return Blocker{}, fmt.Errorf("blockers: insert: %w", err)
	}
	if b.ID, err = res.LastInsertId(); err != nil {
		return Blocker{}, fmt.Errorf("blockers: read id: %w", err)
	}
	q.record(b, "blocker.opened", map[string]any{"blocker_id": b.ID, "kind": b.Kind, "summary": b.Summary})
	q.notify(b)
	return b, nil
}

func (q *Queue) notify(b Blocker) {
	if q.notifier == nil {
		return
	}
	if err := q.notifier.Notify(b); err != nil {
		q.record(b, "notify.failed", map[string]any{"blocker_id": b.ID, "error": err.Error()})
	}
}

// record journals a blocker event. The blocker row is already the durable record,
// so a journal failure must not undo it; it is logged loudly instead.
func (q *Queue) record(b Blocker, kind string, payload map[string]any) {
	if _, err := q.journal.Append(journal.Event{ProjectID: b.ProjectID, Seat: b.Seat, Type: kind, Payload: payload}); err != nil {
		q.log.Error("journal append failed", "component", "blockers", "event", kind, "blocker_id", b.ID, "error", err)
	}
}

// Answer records the user's choice and text. choice must be one of the options when the
// blocker has any.
func (q *Queue) Answer(id int64, choice, text string) (Blocker, error) {
	b, err := q.Get(id)
	if err != nil {
		return Blocker{}, err
	}
	if b.AnsweredAt != nil {
		return Blocker{}, ErrAnswered
	}
	if len(b.Options) > 0 && !slices.Contains(b.Options, choice) {
		return Blocker{}, fmt.Errorf("blockers: %q is not one of %v", choice, b.Options)
	}
	if len(b.Options) == 0 && text == "" {
		return Blocker{}, fmt.Errorf("blockers: a blocker without options needs a text answer")
	}
	at := q.now().UTC()
	if _, err := q.db.Exec(`UPDATE blockers SET answer_choice = ?, answer_text = ?, answered_at = ? WHERE id = ?`,
		choice, text, at.Format(time.RFC3339Nano), id); err != nil {
		return Blocker{}, fmt.Errorf("blockers: update: %w", err)
	}
	b.Choice, b.Text, b.AnsweredAt = choice, text, &at
	q.record(b, "blocker.answered", map[string]any{"blocker_id": id, "choice": choice})
	return b, nil
}

// Get returns one blocker.
func (q *Queue) Get(id int64) (Blocker, error) {
	rows, err := q.db.Query(selectBlockers+` WHERE id = ?`, id)
	if err != nil {
		return Blocker{}, fmt.Errorf("blockers: query: %w", err)
	}
	list, err := scanAll(rows)
	if err != nil {
		return Blocker{}, err
	}
	if len(list) == 0 {
		return Blocker{}, fmt.Errorf("blockers: no blocker %d", id)
	}
	return list[0], nil
}

// List returns blockers, newest first. openOnly drops answered ones.
func (q *Queue) List(openOnly bool) ([]Blocker, error) {
	rows, err := q.db.Query(selectBlockers+` WHERE (? = 0 OR answered_at IS NULL) ORDER BY id DESC`, openOnly)
	if err != nil {
		return nil, fmt.Errorf("blockers: query: %w", err)
	}
	return scanAll(rows)
}

const selectBlockers = `SELECT id, project_id, seat, kind, summary, detail, options, opened_at,
	COALESCE(answer_choice, ''), COALESCE(answer_text, ''), answered_at FROM blockers`

func scanAll(rows *sql.Rows) ([]Blocker, error) {
	defer func() { _ = rows.Close() }() // why: rows.Err() reports iteration failures
	var out []Blocker
	for rows.Next() {
		b, err := scanOne(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func scanOne(rows *sql.Rows) (Blocker, error) {
	var b Blocker
	var options, opened string
	var answered sql.NullString
	if err := rows.Scan(&b.ID, &b.ProjectID, &b.Seat, &b.Kind, &b.Summary, &b.Detail, &options,
		&opened, &b.Choice, &b.Text, &answered); err != nil {
		return Blocker{}, fmt.Errorf("blockers: scan: %w", err)
	}
	if err := json.Unmarshal([]byte(options), &b.Options); err != nil {
		return Blocker{}, fmt.Errorf("blockers: decode options of %d: %w", b.ID, err)
	}
	var err error
	if b.OpenedAt, err = time.Parse(time.RFC3339Nano, opened); err != nil {
		return Blocker{}, fmt.Errorf("blockers: parse opened_at of %d: %w", b.ID, err)
	}
	if answered.Valid {
		at, err := time.Parse(time.RFC3339Nano, answered.String)
		if err != nil {
			return Blocker{}, fmt.Errorf("blockers: parse answered_at of %d: %w", b.ID, err)
		}
		b.AnsweredAt = &at
	}
	return b, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
