// Package store opens the instance database that every other package shares.
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // registers the pure-Go "sqlite" driver
)

const schema = `
CREATE TABLE IF NOT EXISTS events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ts TEXT NOT NULL,
	project_id TEXT NOT NULL,
	doctrine_version TEXT NOT NULL,
	seat TEXT NOT NULL,
	ticket_id TEXT,
	type TEXT NOT NULL,
	payload TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS events_project ON events(project_id, id);
CREATE TABLE IF NOT EXISTS blockers (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	seat TEXT NOT NULL,
	kind TEXT NOT NULL,
	summary TEXT NOT NULL,
	detail TEXT NOT NULL,
	options TEXT NOT NULL,
	opened_at TEXT NOT NULL,
	answer_choice TEXT,
	answer_text TEXT,
	answered_at TEXT
);
CREATE TABLE IF NOT EXISTS inbox (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	to_seat TEXT NOT NULL,
	from_seat TEXT NOT NULL,
	kind TEXT NOT NULL,
	body TEXT NOT NULL,
	created_at TEXT NOT NULL,
	delivered_at TEXT
);
CREATE TABLE IF NOT EXISTS transcript (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ts TEXT NOT NULL,
	speaker TEXT NOT NULL,
	text TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
	seat_key TEXT PRIMARY KEY,
	session_id TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS caps (
	project_id TEXT PRIMARY KEY,
	cap_usd REAL NOT NULL
);
CREATE TABLE IF NOT EXISTS fleets (
	project_id TEXT PRIMARY KEY,
	state TEXT NOT NULL,
	reason TEXT NOT NULL
);`

// Open opens (creating if needed) the SQLite database at path and applies the schema.
func Open(path string) (*sql.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("store: empty database path")
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	// SQLite allows one writer; a single connection keeps writes serialized.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		closeErr := db.Close()
		return nil, fmt.Errorf("store: apply schema: %w (close: %v)", err, closeErr)
	}
	return db, nil
}
