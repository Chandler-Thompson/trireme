package fleet

import (
	"errors"
	"slices"
	"testing"

	"github.com/Chandler-Thompson/trireme/internal/testutil"
)

func newRegistry(t *testing.T) (*Registry, *testutil.Env) {
	env := testutil.New(t)
	return New(env.DB, env.Journal), env
}

func TestGet_UnknownProject_ReturnsErrUnknown(t *testing.T) {
	r, _ := newRegistry(t)
	if _, err := r.Get("nope"); !errors.Is(err, ErrUnknown) {
		t.Fatalf("got %v", err)
	}
}

func TestLifecycle_FullPath_JournalsEveryTransition(t *testing.T) {
	r, env := newRegistry(t)
	steps := []func() error{
		func() error { return r.Start("p") },
		func() error { _, err := r.Pause("p", ReasonUsageWindow); return err },
		func() error { return r.Resume("p") },
		func() error { return r.Stop("p") },
		func() error { return r.Start("p") },
		func() error { return r.Stop("p") },
		func() error { return r.Archive("p") },
	}
	for i, step := range steps {
		if err := step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	want := []string{"fleet.started", "fleet.paused", "fleet.resumed", "fleet.stopped", "fleet.started", "fleet.stopped", "fleet.archived"}
	if !slices.Equal(env.Types(t), want) {
		t.Fatalf("journal %v", env.Types(t))
	}
}

func TestPause_AlreadyPaused_IsNoOpAndKeepsFirstReason(t *testing.T) {
	r, env := newRegistry(t)
	if err := r.Start("p"); err != nil {
		t.Fatal(err)
	}
	if changed, err := r.Pause("p", ReasonProjectCap); !changed || err != nil {
		t.Fatalf("first pause %v %v", changed, err)
	}
	if changed, err := r.Pause("p", ReasonUsageWindow); changed || err != nil {
		t.Fatalf("second pause %v %v", changed, err)
	}
	s, _ := r.Get("p")
	if s.Reason != ReasonProjectCap || len(env.Types(t)) != 2 {
		t.Fatalf("state %+v journal %v", s, env.Types(t))
	}
}

func TestTransitions_FromWrongState_ReturnErrors(t *testing.T) {
	r, _ := newRegistry(t)
	if _, err := r.Pause("p", ReasonUser); !errors.Is(err, ErrUnknown) {
		t.Fatalf("pause unknown: %v", err)
	}
	if err := r.Resume("p"); !errors.Is(err, ErrUnknown) {
		t.Fatalf("resume unknown: %v", err)
	}
	if err := r.Stop("p"); !errors.Is(err, ErrUnknown) {
		t.Fatalf("stop unknown: %v", err)
	}
	if err := r.Start("p"); err != nil {
		t.Fatal(err)
	}
	for name, err := range map[string]error{
		"start running":   r.Start("p"),
		"resume running":  r.Resume("p"),
		"archive running": r.Archive("p"),
	} {
		if err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if err := r.Stop("p"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Pause("p", ReasonUser); err == nil {
		t.Error("pause stopped: expected error")
	}
	if err := r.Stop("p"); err == nil {
		t.Error("stop stopped: expected error")
	}
}

func TestList_ArchivedFleet_IsHidden(t *testing.T) {
	r, _ := newRegistry(t)
	for _, p := range []string{"a", "b"} {
		if err := r.Start(p); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Stop("a"); err != nil {
		t.Fatal(err)
	}
	if err := r.Archive("a"); err != nil {
		t.Fatal(err)
	}
	got, err := r.List()
	if err != nil || len(got) != 1 || got[0].ProjectID != "b" {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestRegistry_ClosedDatabase_ReturnsErrors(t *testing.T) {
	r, env := newRegistry(t)
	env.DB.Close()
	if _, err := r.Get("p"); err == nil || errors.Is(err, ErrUnknown) {
		t.Fatalf("get: %v", err)
	}
	if _, err := r.List(); err == nil {
		t.Fatal("list: expected error")
	}
	if err := r.Start("p"); err == nil {
		t.Fatal("start: expected error")
	}
}

func TestSet_JournalRefuses_ReturnsError(t *testing.T) {
	r, env := newRegistry(t)
	if _, err := env.DB.Exec(`CREATE TRIGGER no_events BEFORE INSERT ON events BEGIN SELECT RAISE(FAIL, 'full'); END`); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("p"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSet_WriteRefused_ReturnsError(t *testing.T) {
	r, env := newRegistry(t)
	if _, err := env.DB.Exec(`CREATE TRIGGER no_fleets BEFORE INSERT ON fleets BEGIN SELECT RAISE(FAIL, 'ro'); END`); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("p"); err == nil {
		t.Fatal("expected error")
	}
}
