package inbox

import (
	"slices"
	"testing"

	"github.com/Chandler-Thompson/trireme/internal/testutil"
)

func newInbox(t *testing.T) (*Inbox, *testutil.Env) {
	env := testutil.New(t)
	return New(env.DB, env.Journal, env.Clock.Now), env
}

func TestPost_UnknownKind_ReturnsError(t *testing.T) {
	in, _ := newInbox(t)
	if _, err := in.Post(Item{ProjectID: "p", ToSeat: "a", FromSeat: "b", Kind: "gossip"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestPost_MissingAddress_ReturnsError(t *testing.T) {
	in, _ := newInbox(t)
	for _, it := range []Item{{ToSeat: "a", FromSeat: "b"}, {ProjectID: "p", FromSeat: "b"}, {ProjectID: "p", ToSeat: "a"}} {
		it.Kind = KindConsult
		if _, err := in.Post(it); err == nil {
			t.Errorf("expected error for %+v", it)
		}
	}
}

func TestPost_EachKind_JournalsMatchingEvent(t *testing.T) {
	in, env := newInbox(t)
	for _, k := range []string{KindDirective, KindConsult, KindAnswer, KindKick} {
		if _, err := in.Post(Item{ProjectID: "p", ToSeat: "technical-lead", FromSeat: "xo", Kind: k, Body: "b"}); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"directive.posted", "consult.opened", "consult.answered", "seat.kicked"}
	if !slices.Equal(env.Types(t), want) {
		t.Fatalf("journal %v", env.Types(t))
	}
}

func TestPending_AfterDelivery_ReturnsOnlyUndeliveredForThatSeat(t *testing.T) {
	in, _ := newInbox(t)
	first, _ := in.Post(Item{ProjectID: "p", ToSeat: "a", FromSeat: "xo", Kind: KindDirective, Body: "1"})
	if _, err := in.Post(Item{ProjectID: "p", ToSeat: "a", FromSeat: "xo", Kind: KindDirective, Body: "2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := in.Post(Item{ProjectID: "p", ToSeat: "b", FromSeat: "xo", Kind: KindDirective, Body: "3"}); err != nil {
		t.Fatal(err)
	}
	if err := in.MarkDelivered([]Item{first}); err != nil {
		t.Fatal(err)
	}
	got, err := in.Pending("p", "a")
	if err != nil || len(got) != 1 || got[0].Body != "2" {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestPost_JournalRefuses_StoresNothing(t *testing.T) {
	in, env := newInbox(t)
	if _, err := env.DB.Exec(`CREATE TRIGGER no_events BEFORE INSERT ON events BEGIN SELECT RAISE(FAIL, 'full'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := in.Post(Item{ProjectID: "p", ToSeat: "a", FromSeat: "b", Kind: KindConsult}); err == nil {
		t.Fatal("expected error")
	}
	if got, _ := in.Pending("p", "a"); len(got) != 0 {
		t.Fatalf("an item the journal refused must not be stored: %+v", got)
	}
}

func TestPost_InsertFails_ReturnsError(t *testing.T) {
	in, env := newInbox(t)
	if _, err := env.DB.Exec(`DROP TABLE inbox`); err != nil {
		t.Fatal(err)
	}
	if _, err := in.Post(Item{ProjectID: "p", ToSeat: "a", FromSeat: "b", Kind: KindConsult}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := in.Pending("p", "a"); err == nil {
		t.Fatal("expected error")
	}
	if err := in.MarkDelivered([]Item{{ID: 1}}); err == nil {
		t.Fatal("expected error")
	}
}

func TestPending_CorruptTimestamp_ReturnsError(t *testing.T) {
	in, env := newInbox(t)
	if _, err := env.DB.Exec(`INSERT INTO inbox (project_id, to_seat, from_seat, kind, body, created_at) VALUES ('p','a','b','consult','','nope')`); err != nil {
		t.Fatal(err)
	}
	if _, err := in.Pending("p", "a"); err == nil {
		t.Fatal("expected error")
	}
}
