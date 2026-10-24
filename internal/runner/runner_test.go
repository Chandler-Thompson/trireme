package runner

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Chandler-Thompson/trireme/internal/fleet"
	"github.com/Chandler-Thompson/trireme/internal/harness"
	"github.com/Chandler-Thompson/trireme/internal/inbox"
	"github.com/Chandler-Thompson/trireme/internal/seat"
	"github.com/Chandler-Thompson/trireme/internal/testutil"
)

type fakeSeat struct {
	replies []string
	err     error
	prompts []string
}

func (f *fakeSeat) Turn(_ context.Context, prompt, _ string) (harness.Reply, error) {
	f.prompts = append(f.prompts, prompt)
	if f.err != nil {
		return harness.Reply{}, f.err
	}
	text := "STATUS: IDLE"
	if len(f.replies) > 0 {
		text, f.replies = f.replies[0], f.replies[1:]
	}
	return harness.Reply{Text: text}, nil
}

type rig struct {
	env    *testutil.Env
	in     *inbox.Inbox
	fleets *fleet.Registry
	r      *Runner
}

func newRig(t *testing.T, seats map[string]*fakeSeat, order []string) *rig {
	env := testutil.New(t)
	g := &rig{env: env, in: inbox.New(env.DB, env.Journal, env.Clock.Now), fleets: fleet.New(env.DB, env.Journal)}
	if err := g.fleets.Start("p"); err != nil {
		t.Fatal(err)
	}
	var slots []*Slot
	for _, id := range order {
		slots = append(slots, &Slot{ProjectID: "p", SeatID: id, Seat: seats[id]})
	}
	g.r = New(slots, g.in, g.fleets, env.Clock.Now, time.Minute, env.Log)
	return g
}

func (g *rig) post(t *testing.T, to, body string) {
	t.Helper()
	if _, err := g.in.Post(inbox.Item{ProjectID: "p", ToSeat: to, FromSeat: "xo", Kind: inbox.KindDirective, Body: body}); err != nil {
		t.Fatal(err)
	}
}

func TestStep_NothingPending_RunsNoTurn(t *testing.T) {
	a := &fakeSeat{}
	g := newRig(t, map[string]*fakeSeat{"a": a}, []string{"a"})
	if ran, err := g.r.Step(context.Background()); ran || err != nil {
		t.Fatalf("ran %v err %v", ran, err)
	}
}

func TestStep_InboxItem_DeliversItInPromptAndMarksDelivered(t *testing.T) {
	a := &fakeSeat{}
	g := newRig(t, map[string]*fakeSeat{"a": a}, []string{"a"})
	g.post(t, "a", "ship the journal")
	if ran, err := g.r.Step(context.Background()); !ran || err != nil {
		t.Fatalf("ran %v err %v", ran, err)
	}
	if !strings.Contains(a.prompts[0], "directive from xo: ship the journal") {
		t.Fatalf("prompt %q", a.prompts[0])
	}
	if pending, _ := g.in.Pending("p", "a"); len(pending) != 0 {
		t.Fatal("item not marked delivered")
	}
}

func TestStep_SeatSaysContinue_GetsAnotherTurnWithoutInput(t *testing.T) {
	a := &fakeSeat{replies: []string{"working\nSTATUS: CONTINUE", "done\nSTATUS: IDLE"}}
	g := newRig(t, map[string]*fakeSeat{"a": a}, []string{"a"})
	g.post(t, "a", "go")
	for i := 0; i < 3; i++ {
		if _, err := g.r.Step(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(a.prompts) != 2 || !strings.Contains(a.prompts[1], "No new items") {
		t.Fatalf("prompts %v", a.prompts)
	}
	if g.r.Statuses()["p/a"] != StatusIdle {
		t.Fatalf("statuses %v", g.r.Statuses())
	}
}

func TestStep_TwoReadySeats_AlternatesRoundRobin(t *testing.T) {
	a := &fakeSeat{replies: []string{"STATUS: CONTINUE", "STATUS: CONTINUE"}}
	b := &fakeSeat{replies: []string{"STATUS: CONTINUE", "STATUS: CONTINUE"}}
	g := newRig(t, map[string]*fakeSeat{"a": a, "b": b}, []string{"a", "b"})
	g.post(t, "a", "x")
	g.post(t, "b", "y")
	for i := 0; i < 4; i++ {
		if _, err := g.r.Step(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(a.prompts) != 2 || len(b.prompts) != 2 {
		t.Fatalf("a %d b %d", len(a.prompts), len(b.prompts))
	}
}

func TestStep_PausedFleet_RunsNoTurn(t *testing.T) {
	a := &fakeSeat{}
	g := newRig(t, map[string]*fakeSeat{"a": a}, []string{"a"})
	g.post(t, "a", "x")
	if _, err := g.fleets.Pause("p", fleet.ReasonUser); err != nil {
		t.Fatal(err)
	}
	if ran, _ := g.r.Step(context.Background()); ran {
		t.Fatal("ran a turn in a paused fleet")
	}
}

func TestStep_TurnFails_KeepsItemsAndBacksOff(t *testing.T) {
	a := &fakeSeat{err: errors.New("boom")}
	g := newRig(t, map[string]*fakeSeat{"a": a}, []string{"a"})
	g.post(t, "a", "x")
	if ran, err := g.r.Step(context.Background()); !ran || err == nil {
		t.Fatalf("ran %v err %v", ran, err)
	}
	if pending, _ := g.in.Pending("p", "a"); len(pending) != 1 {
		t.Fatal("failed turn must leave items pending")
	}
	if ran, _ := g.r.Step(context.Background()); ran {
		t.Fatal("seat must back off after a failure")
	}
	g.env.Clock.T = g.env.Clock.T.Add(2 * time.Minute)
	if ran, _ := g.r.Step(context.Background()); !ran {
		t.Fatal("seat must retry after the backoff")
	}
}

func TestStep_UsageLimited_ReportsNoError(t *testing.T) {
	for _, e := range []error{seat.ErrUsageLimited, seat.ErrNotRunning} {
		a := &fakeSeat{err: e}
		g := newRig(t, map[string]*fakeSeat{"a": a}, []string{"a"})
		g.post(t, "a", "x")
		if ran, err := g.r.Step(context.Background()); !ran || err != nil {
			t.Fatalf("%v: ran %v err %v", e, ran, err)
		}
	}
}

func TestStep_InboxBroken_ReturnsError(t *testing.T) {
	g := newRig(t, map[string]*fakeSeat{"a": {}}, []string{"a"})
	if _, err := g.env.DB.Exec(`DROP TABLE inbox`); err != nil {
		t.Fatal(err)
	}
	if _, err := g.r.Step(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoop_ContextCancelled_Returns(t *testing.T) {
	a := &fakeSeat{err: errors.New("boom")}
	g := newRig(t, map[string]*fakeSeat{"a": a}, []string{"a"})
	g.post(t, "a", "x")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() { g.r.Loop(ctx, 5*time.Millisecond); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("loop did not stop")
	}
}

func TestParseStatus_Variants_ReturnsLastStatusOrIdle(t *testing.T) {
	cases := map[string]string{
		"":                                    StatusIdle,
		"no status here":                      StatusIdle,
		"STATUS: CONTINUE":                    StatusContinue,
		"a\nSTATUS: BLOCKED\nb\nSTATUS: IDLE": StatusIdle,
		"  STATUS:  BLOCKED  ":                StatusBlocked,
		"STATUS: MAYBE":                       StatusIdle,
		"text STATUS: CONTINUE":               StatusIdle,
	}
	for in, want := range cases {
		if got := ParseStatus(in); got != want {
			t.Errorf("ParseStatus(%q) = %s, want %s", in, got, want)
		}
	}
}
