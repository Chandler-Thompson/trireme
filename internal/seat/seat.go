// Package seat runs turns of one seat's long-lived session (doctrine module "seats"),
// recording spend, compacting at the threshold, and pausing the fleet on limits.
package seat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Chandler-Thompson/trireme/internal/blockers"
	"github.com/Chandler-Thompson/trireme/internal/budget"
	"github.com/Chandler-Thompson/trireme/internal/fleet"
	"github.com/Chandler-Thompson/trireme/internal/harness"
	"github.com/Chandler-Thompson/trireme/internal/journal"
)

// ErrNotRunning is returned when a turn is asked of a seat whose fleet is not running.
var ErrNotRunning = errors.New("seat: fleet is not running")

// ErrUsageLimited is returned when the harness's usage window is spent.
var ErrUsageLimited = errors.New("seat: usage window spent")

// Config describes one seat.
type Config struct {
	ProjectID       string
	Seat            string
	DoctrineVersion string
	// Card is the seat's role card, sent as its appended system prompt.
	Card string
	Spec harness.SessionSpec
	// CompactAt is the context size at which the session is compacted. 0 disables it.
	CompactAt int64
}

// Deps are the services a seat writes to. Fleets, Blockers and Caps are nil for a seat
// that belongs to no fleet, which is the XO.
type Deps struct {
	Adapter  harness.Adapter
	Journal  *journal.Journal
	Fleets   *fleet.Registry
	Blockers *blockers.Queue
	Caps     *budget.Caps
	Sessions *Sessions
	Log      *slog.Logger
}

// Seat is one staffed seat.
type Seat struct {
	cfg Config
	d   Deps
}

// New returns a Seat.
func New(cfg Config, d Deps) *Seat { return &Seat{cfg: cfg, d: d} }

// Key identifies the seat's session across restarts.
func (s *Seat) Key() string { return s.cfg.ProjectID + "/" + s.cfg.Seat }

// Turn runs one turn of the seat's session with prompt, charging spend to ticket.
func (s *Seat) Turn(ctx context.Context, prompt, ticket string) (harness.Reply, error) {
	if err := s.checkRunning(); err != nil {
		return harness.Reply{}, err
	}
	spec, isNew, err := s.spec()
	if err != nil {
		return harness.Reply{}, err
	}
	reply, err := s.d.Adapter.Turn(ctx, spec, prompt)
	if err != nil {
		s.record("seat.error", ticket, map[string]any{"reason": err.Error()})
		return harness.Reply{}, err
	}
	if reply.UsageLimited {
		s.pause(fleet.ReasonUsageWindow, "Fleet paused: the usage window is spent")
		return reply, ErrUsageLimited
	}
	if err := s.afterTurn(reply, isNew, ticket); err != nil {
		return reply, err
	}
	s.maybeCompact(ctx, reply, ticket)
	return reply, nil
}

func (s *Seat) checkRunning() error {
	if s.d.Fleets == nil {
		return nil
	}
	st, err := s.d.Fleets.Get(s.cfg.ProjectID)
	if err != nil {
		return err
	}
	if st.State != fleet.Running {
		return ErrNotRunning
	}
	return nil
}

func (s *Seat) spec() (harness.SessionSpec, bool, error) {
	spec := s.cfg.Spec
	id, err := s.d.Sessions.Get(s.Key())
	if err != nil {
		return spec, false, err
	}
	spec.SessionID = id
	spec.SystemPrompt = s.cfg.Card
	return spec, id == "", nil
}

func (s *Seat) afterTurn(reply harness.Reply, isNew bool, ticket string) error {
	if isNew {
		if err := s.d.Sessions.Put(s.Key(), reply.SessionID); err != nil {
			return err
		}
		s.record("seat.started", ticket, map[string]any{"session_id": reply.SessionID, "harness": s.d.Adapter.Name()})
	}
	sp := reply.Spend
	if err := s.mustRecord("model.call", ticket, map[string]any{"harness": s.d.Adapter.Name(), "model": sp.Model,
		"tokens_in": sp.TokensIn, "tokens_in_fresh": sp.TokensInFresh, "tokens_out": sp.TokensOut,
		"context_tokens": sp.ContextTokens, "cost_usd": sp.CostUSD}); err != nil {
		return fmt.Errorf("seat: record spend: %w", err)
	}
	return s.checkCap()
}

func (s *Seat) checkCap() error {
	if s.d.Caps == nil || s.d.Fleets == nil {
		return nil
	}
	over, spent, usd, err := s.d.Caps.Over(s.cfg.ProjectID)
	if err != nil {
		return err
	}
	if over {
		s.pause(fleet.ReasonProjectCap,
			fmt.Sprintf("Fleet paused: the project cap of $%.2f is reached ($%.2f spent)", usd, spent))
	}
	return nil
}

// compactionPrompt asks the seat to write its compaction log entry before it forgets.
const compactionPrompt = `Your context is about to be compacted. Before that happens, append a compaction entry to your task log under docs/%s/log/: what you are carrying forward, written once for an agent to reload and once as a few plain sentences for the user. Then reply with the single word DONE.`

func (s *Seat) maybeCompact(ctx context.Context, reply harness.Reply, ticket string) {
	if s.cfg.CompactAt <= 0 || reply.Spend.ContextTokens < s.cfg.CompactAt {
		return
	}
	spec, _, err := s.spec()
	if err == nil {
		_, err = s.d.Adapter.Turn(ctx, spec, fmt.Sprintf(compactionPrompt, s.cfg.Seat))
	}
	if err == nil {
		_, err = s.d.Adapter.Compact(ctx, spec)
	}
	if err != nil {
		s.record("seat.error", ticket, map[string]any{"reason": "compaction: " + err.Error()})
		return
	}
	s.record("seat.compacted", ticket, map[string]any{"context_tokens": reply.Spend.ContextTokens})
}

func (s *Seat) pause(reason, summary string) {
	if s.d.Fleets == nil {
		return
	}
	changed, err := s.d.Fleets.Pause(s.cfg.ProjectID, reason)
	if err != nil {
		s.d.Log.Error("pause failed", "component", "seat", "project", s.cfg.ProjectID, "reason", reason, "error", err)
		return
	}
	if !changed || s.d.Blockers == nil {
		return
	}
	b := blockers.Blocker{ProjectID: s.cfg.ProjectID, Seat: "system", Kind: blockers.KindFleetPaused,
		Summary: summary, Detail: "The fleet stays paused until you resume it.", Options: []string{blockers.ChoiceResume}}
	if reason == fleet.ReasonProjectCap {
		b.Kind, b.Options = blockers.KindCapReached, []string{blockers.ChoiceRaiseCap}
		b.Detail = "Type a new cap in dollars, above what has been spent, then raise it. The fleet resumes when you do."
	}
	if _, err := s.d.Blockers.Open(b); err != nil {
		// The fleet is still paused and shows as paused in the fleet list, so this is safe.
		s.d.Log.Error("pause blocker failed", "component", "seat", "project", s.cfg.ProjectID, "error", err)
	}
}

// record journals an event for this seat. Spend must reach the journal, so callers
// recording spend use mustRecord; for every other event a failure is logged loudly
// rather than failing a turn whose outcome is already known.
func (s *Seat) record(kind, ticket string, payload map[string]any) {
	if err := s.mustRecord(kind, ticket, payload); err != nil {
		s.d.Log.Error("journal append failed", "component", "seat", "seat", s.cfg.Seat,
			"ticket", ticket, "event", kind, "error", err)
	}
}

func (s *Seat) mustRecord(kind, ticket string, payload map[string]any) error {
	_, err := s.d.Journal.Append(journal.Event{ProjectID: s.cfg.ProjectID, DoctrineVersion: s.cfg.DoctrineVersion,
		Seat: s.cfg.Seat, TicketID: ticket, Type: kind, Payload: payload})
	return err
}
