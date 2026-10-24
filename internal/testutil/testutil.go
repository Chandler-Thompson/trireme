// Package testutil builds the shared fixtures package tests need.
package testutil

import (
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/Chandler-Thompson/trireme/internal/journal"
	"github.com/Chandler-Thompson/trireme/internal/store"
)

// Clock is a settable time source.
type Clock struct{ T time.Time }

// Now returns the clock's time.
func (c *Clock) Now() time.Time { return c.T }

// Env is an opened database, a journal and a clock.
type Env struct {
	DB      *sql.DB
	Journal *journal.Journal
	Clock   *Clock
	Log     *slog.Logger
}

// New opens a fresh database in a temp directory, closed when the test ends.
func New(t *testing.T) *Env {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	clock := &Clock{T: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)}
	return &Env{DB: db, Journal: journal.New(db, clock.Now), Clock: clock,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil))}
}

// Types returns the journal's event types in order.
func (e *Env) Types(t *testing.T) []string {
	t.Helper()
	events, err := e.Journal.Since("", 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, ev := range events {
		out = append(out, ev.Type)
	}
	return out
}
