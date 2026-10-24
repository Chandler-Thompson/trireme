package seat

import (
	"database/sql"
	"errors"
	"fmt"
)

// Sessions remembers each seat's harness session id, so a long-lived session survives
// a server restart.
type Sessions struct{ db *sql.DB }

// NewSessions returns a session store over the instance database.
func NewSessions(db *sql.DB) *Sessions { return &Sessions{db: db} }

// Get returns the stored session id, or "" when the seat has none yet.
func (s *Sessions) Get(key string) (string, error) {
	var id string
	err := s.db.QueryRow(`SELECT session_id FROM sessions WHERE seat_key = ?`, key).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("seat: read session %s: %w", key, err)
	}
	return id, nil
}

// Put stores a seat's session id.
func (s *Sessions) Put(key, id string) error {
	if _, err := s.db.Exec(`INSERT INTO sessions (seat_key, session_id) VALUES (?, ?)
		ON CONFLICT(seat_key) DO UPDATE SET session_id = excluded.session_id`, key, id); err != nil {
		return fmt.Errorf("seat: store session %s: %w", key, err)
	}
	return nil
}

// Forget drops a seat's session id, so its next turn starts a fresh session.
func (s *Sessions) Forget(key string) error {
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE seat_key = ?`, key); err != nil {
		return fmt.Errorf("seat: forget session %s: %w", key, err)
	}
	return nil
}
