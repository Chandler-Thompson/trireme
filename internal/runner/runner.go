// Package runner decides which seat takes the next turn. Seats take turns one at a
// time across the whole instance, which keeps a shared usage window from being spent
// in parallel and keeps two seats from editing the same checkout at once.
package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Chandler-Thompson/trireme/internal/fleet"
	"github.com/Chandler-Thompson/trireme/internal/harness"
	"github.com/Chandler-Thompson/trireme/internal/inbox"
	"github.com/Chandler-Thompson/trireme/internal/seat"
)

// Statuses a seat reports at the end of each turn.
const (
	StatusContinue = "CONTINUE"
	StatusIdle     = "IDLE"
	StatusBlocked  = "BLOCKED"
)

var statusLine = regexp.MustCompile(`(?m)^\s*STATUS:\s*(CONTINUE|IDLE|BLOCKED)\s*$`)

// Turner is the part of a seat the runner needs.
type Turner interface {
	Turn(ctx context.Context, prompt, ticket string) (harness.Reply, error)
}

// Slot is one seat the runner schedules.
type Slot struct {
	ProjectID string
	SeatID    string
	Seat      Turner
	status    string
	notBefore time.Time
}

// Runner schedules turns.
type Runner struct {
	mu      sync.Mutex
	slots   []*Slot
	next    int
	inbox   *inbox.Inbox
	fleets  *fleet.Registry
	now     func() time.Time
	backoff time.Duration
	log     *slog.Logger
}

// New returns a Runner over the given slots.
func New(slots []*Slot, in *inbox.Inbox, fleets *fleet.Registry, now func() time.Time, backoff time.Duration, log *slog.Logger) *Runner {
	for _, s := range slots {
		s.status = StatusIdle
	}
	return &Runner{slots: slots, inbox: in, fleets: fleets, now: now, backoff: backoff, log: log}
}

// Statuses returns each slot's last reported status, keyed "project/seat".
func (r *Runner) Statuses() map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(r.slots))
	for _, s := range r.slots {
		out[s.ProjectID+"/"+s.SeatID] = s.status
	}
	return out
}

// Step runs at most one turn and reports whether it ran one.
func (r *Runner) Step(ctx context.Context) (bool, error) {
	slot, items, err := r.pick()
	if err != nil || slot == nil {
		return false, err
	}
	reply, err := slot.Seat.Turn(ctx, Prompt(slot.ProjectID, slot.SeatID, items), "")
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		slot.notBefore = r.now().Add(r.backoff)
		if errors.Is(err, seat.ErrUsageLimited) || errors.Is(err, seat.ErrNotRunning) {
			return true, nil
		}
		return true, fmt.Errorf("runner: %s/%s: %w", slot.ProjectID, slot.SeatID, err)
	}
	slot.status = ParseStatus(reply.Text)
	return true, r.inbox.MarkDelivered(items)
}

// pick returns the next slot, in round-robin order, that is in a running fleet, is not
// backing off, and has inbox items or said it would continue.
func (r *Runner) pick() (*Slot, []inbox.Item, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := 0; i < len(r.slots); i++ {
		slot := r.slots[(r.next+i)%len(r.slots)]
		items, ready, err := r.ready(slot)
		if err != nil {
			return nil, nil, err
		}
		if ready {
			r.next = (r.next + i + 1) % len(r.slots)
			return slot, items, nil
		}
	}
	return nil, nil, nil
}

func (r *Runner) ready(slot *Slot) ([]inbox.Item, bool, error) {
	if r.now().Before(slot.notBefore) {
		return nil, false, nil
	}
	st, err := r.fleets.Get(slot.ProjectID)
	if err != nil || st.State != fleet.Running {
		return nil, false, nil // why: an unknown or paused fleet simply has no turn to take
	}
	items, err := r.inbox.Pending(slot.ProjectID, slot.SeatID)
	if err != nil {
		return nil, false, err
	}
	return items, len(items) > 0 || slot.status == StatusContinue, nil
}

// Loop runs Step until ctx ends, sleeping idle between empty steps.
func (r *Runner) Loop(ctx context.Context, idle time.Duration) {
	for ctx.Err() == nil {
		ran, err := r.Step(ctx)
		if err != nil {
			r.log.Error("turn failed", "component", "runner", "error", err)
		}
		if ran {
			continue
		}
		select {
		case <-ctx.Done():
		case <-time.After(idle):
		}
	}
}

// ParseStatus reads the last STATUS line of a reply. A reply without one counts as idle,
// so a seat that forgets the protocol waits for new input instead of looping.
func ParseStatus(text string) string {
	matches := statusLine.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return StatusIdle
	}
	return matches[len(matches)-1][1]
}

// Prompt builds the turn prompt from the seat's inbox.
func Prompt(projectID, seatID string, items []inbox.Item) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Trireme turn] Project %q. You hold the %s seat.\n\n", projectID, seatID)
	if len(items) == 0 {
		b.WriteString("No new items. Continue the work you said you would continue.\n")
	} else {
		b.WriteString("New items for you, oldest first:\n")
		for _, it := range items {
			fmt.Fprintf(&b, "- %s from %s: %s\n", it.Kind, it.FromSeat, it.Body)
		}
	}
	b.WriteString(`
Work per your role card and the doctrine. Use trireme-cli to open blockers for the user,
consult other seats, and read the journal; never message another seat any other way.
End your reply with exactly one line: STATUS: CONTINUE if you have more to do right now,
STATUS: IDLE if you are waiting for new input, or STATUS: BLOCKED if you are waiting on
someone specific.`)
	return b.String()
}
