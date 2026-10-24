// Package xo is the chat between the user and the XO seat. The transcript kept here is a
// record for the user to scroll back through; it is never fed back into the XO's
// session. The XO's continuity comes from its own session and its own memory.
package xo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Chandler-Thompson/trireme/internal/harness"
	"github.com/Chandler-Thompson/trireme/internal/seat"
)

// Speakers in the transcript.
const (
	SpeakerUser   = "user"
	SpeakerXO     = "xo"
	SpeakerSystem = "system"
)

// Line is one transcript entry.
type Line struct {
	ID      int64     `json:"id"`
	TS      time.Time `json:"ts"`
	Speaker string    `json:"speaker"`
	Text    string    `json:"text"`
}

// Turner is the part of a seat the chat needs.
type Turner interface {
	Turn(ctx context.Context, prompt, ticket string) (harness.Reply, error)
}

// Chat runs XO turns one at a time and records the transcript.
type Chat struct {
	turn  sync.Mutex
	seat  Turner
	db    *sql.DB
	now   func() time.Time
	label string
	log   *slog.Logger
}

// New returns a Chat. label is the body tag shown with the XO's lines, for example the
// machine the server runs on.
func New(s Turner, db *sql.DB, now func() time.Time, label string, log *slog.Logger) *Chat {
	return &Chat{seat: s, db: db, now: now, label: label, log: log}
}

// Say records the user's line and returns it. The XO's reply is produced in the
// background and appears in the transcript when it is ready.
func (c *Chat) Say(ctx context.Context, text string) (Line, error) {
	if text == "" {
		return Line{}, fmt.Errorf("xo: empty message")
	}
	line, err := c.add(SpeakerUser, text)
	if err != nil {
		return Line{}, err
	}
	// The reply outlives the request that asked for it, so it must not inherit the
	// request's cancellation.
	go c.reply(context.WithoutCancel(ctx), text)
	return line, nil
}

// reply runs the XO turn for text and records the result, one turn at a time.
func (c *Chat) reply(ctx context.Context, text string) {
	c.turn.Lock()
	defer c.turn.Unlock()
	speaker, out := SpeakerXO, ""
	reply, err := c.seat.Turn(ctx, prompt(c.label, text), "")
	switch {
	case errors.Is(err, seat.ErrUsageLimited):
		speaker, out = SpeakerSystem, "The usage window is spent, so the XO cannot answer until it resets."
	case err != nil:
		speaker, out = SpeakerSystem, "The XO's turn failed: "+err.Error()
	default:
		out = reply.Text
	}
	if _, err := c.add(speaker, out); err != nil {
		c.log.Error("transcript write failed", "component", "xo", "error", err)
	}
}

func prompt(label, text string) string {
	return fmt.Sprintf("[Trireme chat, server on %s] The user says:\n\n%s", label, text)
}

func (c *Chat) add(speaker, text string) (Line, error) {
	l := Line{TS: c.now().UTC(), Speaker: speaker, Text: text}
	res, err := c.db.Exec(`INSERT INTO transcript (ts, speaker, text) VALUES (?, ?, ?)`,
		l.TS.Format(time.RFC3339Nano), speaker, text)
	if err != nil {
		return Line{}, fmt.Errorf("xo: write transcript: %w", err)
	}
	if l.ID, err = res.LastInsertId(); err != nil {
		return Line{}, fmt.Errorf("xo: read id: %w", err)
	}
	return l, nil
}

// Transcript returns lines after afterID, oldest first.
func (c *Chat) Transcript(afterID int64) ([]Line, error) {
	rows, err := c.db.Query(`SELECT id, ts, speaker, text FROM transcript WHERE id > ? ORDER BY id`, afterID)
	if err != nil {
		return nil, fmt.Errorf("xo: read transcript: %w", err)
	}
	defer func() { _ = rows.Close() }() // why: rows.Err() reports iteration failures
	var out []Line
	for rows.Next() {
		var l Line
		var ts string
		if err := rows.Scan(&l.ID, &ts, &l.Speaker, &l.Text); err != nil {
			return nil, fmt.Errorf("xo: scan: %w", err)
		}
		if l.TS, err = time.Parse(time.RFC3339Nano, ts); err != nil {
			return nil, fmt.Errorf("xo: parse ts of %d: %w", l.ID, err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// Busy reports whether an XO turn is running.
func (c *Chat) Busy() bool {
	if c.turn.TryLock() {
		c.turn.Unlock()
		return false
	}
	return true
}
