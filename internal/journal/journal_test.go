package journal

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Chandler-Thompson/trireme/internal/store"
)

func newJournal(t *testing.T) (*Journal, *sql.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "j.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	fixed := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	return New(db, func() time.Time { return fixed }), db
}

func TestAppend_MissingRequiredField_ReturnsError(t *testing.T) {
	j, _ := newJournal(t)
	for _, e := range []Event{
		{Seat: "xo", Type: "fleet.started"},
		{ProjectID: "p", Type: "fleet.started"},
		{ProjectID: "p", Seat: "xo"},
	} {
		if _, err := j.Append(e); err == nil {
			t.Errorf("expected error for %+v", e)
		}
	}
}

func TestAppend_ValidEvent_StampsTimeAndID(t *testing.T) {
	j, _ := newJournal(t)
	got, err := j.Append(Event{ProjectID: "p", Seat: "xo", Type: "fleet.started"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 1 || got.TS.IsZero() || got.Payload == nil {
		t.Fatalf("unexpected event %+v", got)
	}
}

func TestAppend_UnencodablePayload_ReturnsError(t *testing.T) {
	j, _ := newJournal(t)
	_, err := j.Append(Event{ProjectID: "p", Seat: "xo", Type: "x", Payload: map[string]any{"bad": func() {}}})
	if err == nil {
		t.Fatal("expected an encode error")
	}
}

func TestAppend_ClosedDatabase_ReturnsError(t *testing.T) {
	j, db := newJournal(t)
	db.Close()
	if _, err := j.Append(Event{ProjectID: "p", Seat: "xo", Type: "x"}); err == nil {
		t.Fatal("expected an insert error on a closed database")
	}
}

func TestSince_FilteredByProject_ReturnsOnlyThatProjectInOrder(t *testing.T) {
	j, _ := newJournal(t)
	for _, p := range []string{"a", "b", "a"} {
		if _, err := j.Append(Event{ProjectID: p, Seat: "s", Type: "t", TicketID: "T-1",
			Payload: map[string]any{"n": 1}}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := j.Since("a", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 3 || got[0].TicketID != "T-1" {
		t.Fatalf("unexpected events %+v", got)
	}
	if got[0].Payload["n"].(float64) != 1 {
		t.Fatalf("payload not decoded: %+v", got[0].Payload)
	}
}

func TestSince_EmptyProject_ReturnsAllAfterID(t *testing.T) {
	j, _ := newJournal(t)
	for _, p := range []string{"a", "b", "c"} {
		if _, err := j.Append(Event{ProjectID: p, Seat: "s", Type: "t"}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := j.Since("", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ProjectID != "b" {
		t.Fatalf("unexpected events %+v", got)
	}
}

func TestSince_NonPositiveLimit_ReturnsError(t *testing.T) {
	j, _ := newJournal(t)
	if _, err := j.Since("", 0, 0); err == nil {
		t.Fatal("expected an error for limit 0")
	}
}

func TestSince_ClosedDatabase_ReturnsError(t *testing.T) {
	j, db := newJournal(t)
	db.Close()
	if _, err := j.Since("", 0, 1); err == nil {
		t.Fatal("expected a query error")
	}
}

func TestSince_CorruptRows_ReturnsError(t *testing.T) {
	cases := map[string]string{
		"bad ts":      `INSERT INTO events (ts, project_id, doctrine_version, seat, type, payload) VALUES ('nope','p','','s','t','{}')`,
		"bad payload": `INSERT INTO events (ts, project_id, doctrine_version, seat, type, payload) VALUES ('2026-10-23T12:00:00Z','p','','s','t','{')`,
		"null field":  `INSERT INTO events (ts, project_id, doctrine_version, seat, type, payload) VALUES ('2026-10-23T12:00:00Z','p','','s','t','{}'); UPDATE events SET seat = NULL`,
	}
	for name, stmt := range cases {
		t.Run(name, func(t *testing.T) {
			j, db := newJournal(t)
			if name == "null field" {
				// The schema forbids NULL seats, so rebuild the table without the constraint.
				if _, err := db.Exec(`DROP TABLE events; CREATE TABLE events (id INTEGER PRIMARY KEY, ts TEXT, project_id TEXT, doctrine_version TEXT, seat TEXT, ticket_id TEXT, type TEXT, payload TEXT)`); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.Exec(stmt); err != nil {
				t.Fatal(err)
			}
			if _, err := j.Since("", 0, 10); err == nil {
				t.Fatal("expected an error for a corrupt row")
			}
		})
	}
}

func TestSpent_ModelCallsAcrossProjects_SumsOnlyThatProjectsCalls(t *testing.T) {
	j, _ := newJournal(t)
	events := []Event{
		{ProjectID: "p", Seat: "s", Type: "model.call", Payload: map[string]any{"cost_usd": 0.5}},
		{ProjectID: "p", Seat: "s", Type: "model.call", Payload: map[string]any{"cost_usd": 0.25}},
		{ProjectID: "p", Seat: "s", Type: "fleet.started", Payload: map[string]any{"cost_usd": 9}},
		{ProjectID: "q", Seat: "s", Type: "model.call", Payload: map[string]any{"cost_usd": 4}},
	}
	for _, e := range events {
		if _, err := j.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := j.Spent("p")
	if err != nil || got != 0.75 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestSpent_NoCalls_ReturnsZero(t *testing.T) {
	j, _ := newJournal(t)
	if got, err := j.Spent("p"); err != nil || got != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestSpent_ClosedDatabase_ReturnsError(t *testing.T) {
	j, db := newJournal(t)
	db.Close()
	if _, err := j.Spent("p"); err == nil {
		t.Fatal("expected an error")
	}
}
