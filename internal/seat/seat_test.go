package seat

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Chandler-Thompson/trireme/internal/blockers"
	"github.com/Chandler-Thompson/trireme/internal/budget"
	"github.com/Chandler-Thompson/trireme/internal/fleet"
	"github.com/Chandler-Thompson/trireme/internal/harness"
	"github.com/Chandler-Thompson/trireme/internal/testutil"
)

type fakeAdapter struct {
	replies    []harness.Reply
	errs       []error
	prompts    []string
	specs      []harness.SessionSpec
	compacts   int
	compactErr error
}

func (f *fakeAdapter) Name() string { return "fake" }

func (f *fakeAdapter) Turn(_ context.Context, spec harness.SessionSpec, prompt string) (harness.Reply, error) {
	f.prompts = append(f.prompts, prompt)
	f.specs = append(f.specs, spec)
	i := len(f.prompts) - 1
	var err error
	if i < len(f.errs) {
		err = f.errs[i]
	}
	if i < len(f.replies) {
		return f.replies[i], err
	}
	return harness.Reply{SessionID: "sess", Text: "ok"}, err
}

func (f *fakeAdapter) Compact(_ context.Context, _ harness.SessionSpec) (harness.Reply, error) {
	f.compacts++
	return harness.Reply{}, f.compactErr
}

type rig struct {
	env     *testutil.Env
	adapter *fakeAdapter
	fleets  *fleet.Registry
	queue   *blockers.Queue
	seat    *Seat
}

func newRig(t *testing.T, cfg Config, withFleet bool) *rig {
	env := testutil.New(t)
	r := &rig{env: env, adapter: &fakeAdapter{}}
	d := Deps{Adapter: r.adapter, Journal: env.Journal, Sessions: NewSessions(env.DB), Log: env.Log}
	if withFleet {
		d.Caps = budget.New(env.DB, env.Journal)
		if err := d.Caps.Seed(cfg.ProjectID, 1000); err != nil {
			t.Fatal(err)
		}
		r.fleets = fleet.New(env.DB, env.Journal)
		r.queue = blockers.New(env.DB, env.Journal, nil, env.Clock.Now, env.Log)
		d.Fleets, d.Blockers = r.fleets, r.queue
		if err := r.fleets.Start(cfg.ProjectID); err != nil {
			t.Fatal(err)
		}
	}
	if cfg.ProjectID == "" {
		cfg.ProjectID = "instance"
	}
	r.seat = New(cfg, d)
	return r
}

func base() Config {
	return Config{ProjectID: "p", Seat: "technical-lead", Card: "card", Spec: harness.SessionSpec{WorkDir: "/w", Model: "opus"}}
}

func TestTurn_FirstTurn_StartsSessionAndJournalsSpend(t *testing.T) {
	r := newRig(t, base(), true)
	r.adapter.replies = []harness.Reply{{SessionID: "s1", Spend: harness.Spend{Model: "opus", CostUSD: 0.1}}}
	if _, err := r.seat.Turn(context.Background(), "go", "T-1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.seat.d.Sessions.Get("p/technical-lead"); got != "s1" {
		t.Fatalf("session not stored: %q", got)
	}
	if spec := r.adapter.specs[0]; spec.SessionID != "" || spec.SystemPrompt != "card" || spec.WorkDir != "/w" {
		t.Fatalf("spec %+v", spec)
	}
	types := r.env.Types(t)
	if !slices.Equal(types[len(types)-2:], []string{"seat.started", "model.call"}) {
		t.Fatalf("journal %v", types)
	}
	events, _ := r.env.Journal.Since("p", 0, 100)
	if last := events[len(events)-1]; last.TicketID != "T-1" || last.Payload["harness"] != "fake" {
		t.Fatalf("model.call %+v", last)
	}
}

func TestTurn_SecondTurn_ResumesStoredSession(t *testing.T) {
	r := newRig(t, base(), true)
	for i := 0; i < 2; i++ {
		if _, err := r.seat.Turn(context.Background(), "go", ""); err != nil {
			t.Fatal(err)
		}
	}
	if r.adapter.specs[1].SessionID != "sess" {
		t.Fatalf("second turn did not resume: %+v", r.adapter.specs[1])
	}
	if n := strings.Count(strings.Join(r.env.Types(t), ","), "seat.started"); n != 1 {
		t.Fatalf("seat.started %d times", n)
	}
}

func TestTurn_FleetPaused_RefusesWithoutCallingHarness(t *testing.T) {
	r := newRig(t, base(), true)
	if _, err := r.fleets.Pause("p", fleet.ReasonUser); err != nil {
		t.Fatal(err)
	}
	if _, err := r.seat.Turn(context.Background(), "go", ""); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("got %v", err)
	}
	if len(r.adapter.prompts) != 0 {
		t.Fatal("harness must not be called for a paused fleet")
	}
}

func TestTurn_UnknownFleet_ReturnsError(t *testing.T) {
	r := newRig(t, base(), true)
	r.seat.cfg.ProjectID = "other"
	if _, err := r.seat.Turn(context.Background(), "go", ""); !errors.Is(err, fleet.ErrUnknown) {
		t.Fatalf("got %v", err)
	}
}

func TestTurn_UsageLimited_PausesFleetAndOpensOneResumeBlocker(t *testing.T) {
	r := newRig(t, base(), true)
	r.adapter.replies = []harness.Reply{{UsageLimited: true}}
	if _, err := r.seat.Turn(context.Background(), "go", ""); !errors.Is(err, ErrUsageLimited) {
		t.Fatalf("got %v", err)
	}
	st, _ := r.fleets.Get("p")
	open, _ := r.queue.List(true)
	if st.State != fleet.Paused || st.Reason != fleet.ReasonUsageWindow || len(open) != 1 || open[0].Kind != blockers.KindFleetPaused {
		t.Fatalf("state %+v blockers %+v", st, open)
	}
}

func TestTurn_HarnessError_JournalsSeatErrorAndReturnsIt(t *testing.T) {
	r := newRig(t, base(), true)
	r.adapter.errs = []error{errors.New("boom")}
	if _, err := r.seat.Turn(context.Background(), "go", ""); err == nil {
		t.Fatal("expected error")
	}
	if types := r.env.Types(t); types[len(types)-1] != "seat.error" {
		t.Fatalf("journal %v", types)
	}
}

func TestTurn_SpendReachesCap_PausesFleetForProjectCap(t *testing.T) {
	r := newRig(t, base(), true)
	if _, err := r.env.DB.Exec(`UPDATE caps SET cap_usd = 1`); err != nil {
		t.Fatal(err)
	}
	r.adapter.replies = []harness.Reply{{SessionID: "s", Spend: harness.Spend{CostUSD: 0.6}}, {SessionID: "s", Spend: harness.Spend{CostUSD: 0.6}}}
	if _, err := r.seat.Turn(context.Background(), "go", ""); err != nil {
		t.Fatal(err)
	}
	if st, _ := r.fleets.Get("p"); st.State != fleet.Running {
		t.Fatalf("paused too early: %+v", st)
	}
	if _, err := r.seat.Turn(context.Background(), "go", ""); err != nil {
		t.Fatal(err)
	}
	st, _ := r.fleets.Get("p")
	open, _ := r.queue.List(true)
	if st.Reason != fleet.ReasonProjectCap || len(open) != 1 || !strings.Contains(open[0].Summary, "$1.00") || open[0].Kind != blockers.KindCapReached {
		t.Fatalf("state %+v blockers %+v", st, open)
	}
}

func TestTurn_ContextPastThreshold_WritesLogEntryThenCompacts(t *testing.T) {
	cfg := base()
	cfg.CompactAt = 100
	r := newRig(t, cfg, true)
	r.adapter.replies = []harness.Reply{{SessionID: "s", Spend: harness.Spend{ContextTokens: 150}}}
	if _, err := r.seat.Turn(context.Background(), "go", ""); err != nil {
		t.Fatal(err)
	}
	if len(r.adapter.prompts) != 2 || !strings.Contains(r.adapter.prompts[1], "docs/technical-lead/log/") || r.adapter.compacts != 1 {
		t.Fatalf("prompts %v compacts %d", r.adapter.prompts, r.adapter.compacts)
	}
	if types := r.env.Types(t); types[len(types)-1] != "seat.compacted" {
		t.Fatalf("journal %v", types)
	}
}

func TestTurn_ContextUnderThreshold_DoesNotCompact(t *testing.T) {
	cfg := base()
	cfg.CompactAt = 100
	r := newRig(t, cfg, true)
	r.adapter.replies = []harness.Reply{{SessionID: "s", Spend: harness.Spend{ContextTokens: 99}}}
	if _, err := r.seat.Turn(context.Background(), "go", ""); err != nil {
		t.Fatal(err)
	}
	if r.adapter.compacts != 0 || len(r.adapter.prompts) != 1 {
		t.Fatal("compacted under the threshold")
	}
}

func TestTurn_CompactionFails_JournalsSeatErrorButTurnSucceeds(t *testing.T) {
	cfg := base()
	cfg.CompactAt = 1
	r := newRig(t, cfg, true)
	r.adapter.replies = []harness.Reply{{SessionID: "s", Spend: harness.Spend{ContextTokens: 5}}}
	r.adapter.compactErr = errors.New("cannot")
	if _, err := r.seat.Turn(context.Background(), "go", ""); err != nil {
		t.Fatalf("turn must still succeed: %v", err)
	}
	if types := r.env.Types(t); types[len(types)-1] != "seat.error" {
		t.Fatalf("journal %v", types)
	}
}

func TestTurn_NoFleet_RunsUngatedAndUsageLimitDoesNotPanic(t *testing.T) {
	cfg := base()
	cfg.ProjectID = "instance"
	r := newRig(t, cfg, false)
	r.adapter.replies = []harness.Reply{{SessionID: "s", Spend: harness.Spend{CostUSD: 5}}, {UsageLimited: true}}
	if _, err := r.seat.Turn(context.Background(), "hi", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := r.seat.Turn(context.Background(), "hi", ""); !errors.Is(err, ErrUsageLimited) {
		t.Fatalf("got %v", err)
	}
}

func TestTurn_SpendNotJournaled_ReturnsError(t *testing.T) {
	r := newRig(t, base(), true)
	if _, err := r.env.DB.Exec(`CREATE TRIGGER no_calls BEFORE INSERT ON events WHEN NEW.type = 'model.call' BEGIN SELECT RAISE(FAIL, 'full'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.seat.Turn(context.Background(), "go", ""); err == nil {
		t.Fatal("a turn whose spend was not recorded must fail")
	}
}

func TestTurn_SessionStoreBroken_ReturnsError(t *testing.T) {
	r := newRig(t, base(), true)
	if _, err := r.env.DB.Exec(`DROP TABLE sessions`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.seat.Turn(context.Background(), "go", ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestTurn_SessionStoreRefusesWrite_ReturnsError(t *testing.T) {
	r := newRig(t, base(), true)
	if _, err := r.env.DB.Exec(`CREATE TRIGGER ro BEFORE INSERT ON sessions BEGIN SELECT RAISE(FAIL, 'ro'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.seat.Turn(context.Background(), "go", ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestSessions_Forget_ClearsStoredID(t *testing.T) {
	env := testutil.New(t)
	s := NewSessions(env.DB)
	if err := s.Put("k", "id"); err != nil {
		t.Fatal(err)
	}
	if err := s.Forget("k"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get("k"); got != "" || err != nil {
		t.Fatalf("got %q %v", got, err)
	}
	env.DB.Close()
	if err := s.Forget("k"); err == nil {
		t.Fatal("expected error")
	}
}
