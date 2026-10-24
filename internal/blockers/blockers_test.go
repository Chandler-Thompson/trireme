package blockers

import (
	"errors"
	"slices"
	"testing"

	"github.com/Chandler-Thompson/trireme/internal/testutil"
)

type recorder struct {
	got []Blocker
	err error
}

func (r *recorder) Notify(b Blocker) error {
	r.got = append(r.got, b)
	return r.err
}

func newQueue(t *testing.T, n Notifier) (*Queue, *testutil.Env) {
	env := testutil.New(t)
	return New(env.DB, env.Journal, n, env.Clock.Now, env.Log), env
}

func TestOpen_MissingFields_ReturnsError(t *testing.T) {
	q, _ := newQueue(t, nil)
	for _, b := range []Blocker{{Seat: "s", Summary: "x"}, {ProjectID: "p", Summary: "x"}, {ProjectID: "p", Seat: "s"}} {
		if _, err := q.Open(b); err == nil {
			t.Errorf("expected error for %+v", b)
		}
	}
}

func TestOpen_Valid_StoresJournalsAndNotifies(t *testing.T) {
	n := &recorder{}
	q, env := newQueue(t, n)
	b, err := q.Open(Blocker{ProjectID: "p", Seat: "technical-producer", Summary: "Need a seat"})
	if err != nil {
		t.Fatal(err)
	}
	if b.ID != 1 || b.Kind != KindGeneral || len(n.got) != 1 {
		t.Fatalf("unexpected %+v / %d notifications", b, len(n.got))
	}
	if !slices.Equal(env.Types(t), []string{"blocker.opened"}) {
		t.Fatalf("journal %v", env.Types(t))
	}
}

func TestOpen_NotifierFails_StillOpensAndJournalsFailure(t *testing.T) {
	q, env := newQueue(t, &recorder{err: errors.New("offline")})
	if _, err := q.Open(Blocker{ProjectID: "p", Seat: "s", Summary: "x"}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(env.Types(t), []string{"blocker.opened", "notify.failed"}) {
		t.Fatalf("journal %v", env.Types(t))
	}
}

func TestOpen_ClosedDatabase_ReturnsError(t *testing.T) {
	q, env := newQueue(t, nil)
	env.DB.Close()
	if _, err := q.Open(Blocker{ProjectID: "p", Seat: "s", Summary: "x"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestAnswer_ChoiceAmongOptions_RecordsAndJournals(t *testing.T) {
	q, env := newQueue(t, nil)
	b, _ := q.Open(Blocker{ProjectID: "p", Seat: "s", Summary: "x", Options: []string{"Accept", "Decline"}})
	got, err := q.Answer(b.ID, "Accept", "fine")
	if err != nil {
		t.Fatal(err)
	}
	if got.Choice != "Accept" || got.Text != "fine" || got.AnsweredAt == nil {
		t.Fatalf("unexpected %+v", got)
	}
	reread, err := q.Get(b.ID)
	if err != nil || reread.AnsweredAt == nil || reread.Choice != "Accept" {
		t.Fatalf("not persisted: %+v %v", reread, err)
	}
	if env.Types(t)[1] != "blocker.answered" {
		t.Fatalf("journal %v", env.Types(t))
	}
}

func TestAnswer_ChoiceNotAnOption_ReturnsError(t *testing.T) {
	q, _ := newQueue(t, nil)
	b, _ := q.Open(Blocker{ProjectID: "p", Seat: "s", Summary: "x", Options: []string{"Accept"}})
	if _, err := q.Answer(b.ID, "Maybe", ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestAnswer_NoOptionsAndNoText_ReturnsError(t *testing.T) {
	q, _ := newQueue(t, nil)
	b, _ := q.Open(Blocker{ProjectID: "p", Seat: "s", Summary: "x"})
	if _, err := q.Answer(b.ID, "", ""); err == nil {
		t.Fatal("expected error")
	}
	if _, err := q.Answer(b.ID, "", "here is my answer"); err != nil {
		t.Fatal(err)
	}
}

func TestAnswer_AlreadyAnswered_ReturnsErrAnswered(t *testing.T) {
	q, _ := newQueue(t, nil)
	b, _ := q.Open(Blocker{ProjectID: "p", Seat: "s", Summary: "x"})
	if _, err := q.Answer(b.ID, "", "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Answer(b.ID, "", "two"); !errors.Is(err, ErrAnswered) {
		t.Fatalf("got %v", err)
	}
}

func TestAnswer_UnknownID_ReturnsError(t *testing.T) {
	q, _ := newQueue(t, nil)
	if _, err := q.Answer(99, "", "x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestList_OpenOnly_HidesAnsweredNewestFirst(t *testing.T) {
	q, _ := newQueue(t, nil)
	for _, s := range []string{"a", "b", "c"} {
		if _, err := q.Open(Blocker{ProjectID: "p", Seat: "s", Summary: s}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := q.Answer(2, "", "done"); err != nil {
		t.Fatal(err)
	}
	open, err := q.List(true)
	if err != nil || len(open) != 2 || open[0].Summary != "c" || open[1].Summary != "a" {
		t.Fatalf("open %+v %v", open, err)
	}
	all, err := q.List(false)
	if err != nil || len(all) != 3 {
		t.Fatalf("all %+v %v", all, err)
	}
}

func TestList_ClosedDatabase_ReturnsError(t *testing.T) {
	q, env := newQueue(t, nil)
	env.DB.Close()
	if _, err := q.List(false); err == nil {
		t.Fatal("expected error")
	}
	if _, err := q.Get(1); err == nil {
		t.Fatal("expected error")
	}
}

func TestList_CorruptRows_ReturnsError(t *testing.T) {
	for name, stmt := range map[string]string{
		"options":  `INSERT INTO blockers (project_id, seat, kind, summary, detail, options, opened_at) VALUES ('p','s','k','x','','{','2026-10-23T12:00:00Z')`,
		"opened":   `INSERT INTO blockers (project_id, seat, kind, summary, detail, options, opened_at) VALUES ('p','s','k','x','','[]','nope')`,
		"answered": `INSERT INTO blockers (project_id, seat, kind, summary, detail, options, opened_at, answered_at) VALUES ('p','s','k','x','','[]','2026-10-23T12:00:00Z','nope')`,
	} {
		t.Run(name, func(t *testing.T) {
			q, env := newQueue(t, nil)
			if _, err := env.DB.Exec(stmt); err != nil {
				t.Fatal(err)
			}
			if _, err := q.List(false); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestAnswer_ClosedDatabaseAfterGet_ReturnsError(t *testing.T) {
	q, env := newQueue(t, nil)
	b, _ := q.Open(Blocker{ProjectID: "p", Seat: "s", Summary: "x"})
	if _, err := env.DB.Exec(`CREATE TRIGGER no_update BEFORE UPDATE ON blockers BEGIN SELECT RAISE(FAIL, 'locked'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Answer(b.ID, "", "x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestOpen_JournalFails_StillOpens(t *testing.T) {
	q, env := newQueue(t, nil)
	if _, err := env.DB.Exec(`CREATE TRIGGER no_events BEFORE INSERT ON events BEGIN SELECT RAISE(FAIL, 'full'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Open(Blocker{ProjectID: "p", Seat: "s", Summary: "x"}); err != nil {
		t.Fatalf("a journal failure must not lose the blocker: %v", err)
	}
}
