package budget

import (
	"errors"
	"testing"

	"github.com/Chandler-Thompson/trireme/internal/journal"
	"github.com/Chandler-Thompson/trireme/internal/testutil"
)

func newCaps(t *testing.T) (*Caps, *testutil.Env) {
	env := testutil.New(t)
	return New(env.DB, env.Journal), env
}

func spend(t *testing.T, env *testutil.Env, usd float64) {
	t.Helper()
	if _, err := env.Journal.Append(journal.Event{ProjectID: "p", Seat: "s", Type: "model.call", Payload: map[string]any{"cost_usd": usd}}); err != nil {
		t.Fatal(err)
	}
}

func TestSeed_ExistingCap_KeepsRaisedValue(t *testing.T) {
	c, _ := newCaps(t)
	if err := c.Seed("p", 5); err != nil {
		t.Fatal(err)
	}
	if err := c.Raise("p", 9); err != nil {
		t.Fatal(err)
	}
	if err := c.Seed("p", 5); err != nil {
		t.Fatal(err)
	}
	if got, _ := c.Get("p"); got != 9 {
		t.Fatalf("restart reset the raised cap: %v", got)
	}
}

func TestSeed_NonPositive_ReturnsError(t *testing.T) {
	c, _ := newCaps(t)
	if err := c.Seed("p", 0); err == nil {
		t.Fatal("expected error")
	}
}

func TestGet_NoCap_ReturnsErrNoCap(t *testing.T) {
	c, _ := newCaps(t)
	if _, err := c.Get("p"); !errors.Is(err, ErrNoCap) {
		t.Fatalf("got %v", err)
	}
	if err := c.Raise("p", 5); !errors.Is(err, ErrNoCap) {
		t.Fatalf("got %v", err)
	}
	if _, _, _, err := c.Over("p"); !errors.Is(err, ErrNoCap) {
		t.Fatalf("got %v", err)
	}
}

func TestRaise_AtOrBelowCapOrSpend_ReturnsError(t *testing.T) {
	c, env := newCaps(t)
	if err := c.Seed("p", 5); err != nil {
		t.Fatal(err)
	}
	spend(t, env, 7)
	for _, usd := range []float64{4, 5, 6, 7} {
		if err := c.Raise("p", usd); err == nil {
			t.Errorf("raise to %v: expected error", usd)
		}
	}
	if err := c.Raise("p", 8); err != nil {
		t.Fatal(err)
	}
	if types := env.Types(t); types[len(types)-1] != "project_settings.changed" {
		t.Fatalf("journal %v", types)
	}
}

func TestOver_SpendReachesCap_ReportsTrue(t *testing.T) {
	c, env := newCaps(t)
	if err := c.Seed("p", 1); err != nil {
		t.Fatal(err)
	}
	spend(t, env, 0.5)
	if over, _, _, _ := c.Over("p"); over {
		t.Fatal("over too early")
	}
	spend(t, env, 0.5)
	over, spent, usd, err := c.Over("p")
	if !over || spent != 1 || usd != 1 || err != nil {
		t.Fatalf("got %v %v %v %v", over, spent, usd, err)
	}
}

func TestCaps_BrokenDatabase_ReturnsErrors(t *testing.T) {
	c, env := newCaps(t)
	if err := c.Seed("p", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := env.DB.Exec(`CREATE TRIGGER ro BEFORE UPDATE ON caps BEGIN SELECT RAISE(FAIL, 'ro'); END`); err != nil {
		t.Fatal(err)
	}
	if err := c.Raise("p", 5); err == nil {
		t.Fatal("expected update error")
	}
	if _, err := env.DB.Exec(`ALTER TABLE events RENAME TO gone`); err != nil {
		t.Fatal(err)
	}
	if err := c.Raise("p", 5); err == nil {
		t.Fatal("expected spend error")
	}
	if _, _, _, err := c.Over("p"); err == nil {
		t.Fatal("expected spend error")
	}
	env.DB.Close()
	if err := c.Seed("q", 1); err == nil {
		t.Fatal("expected seed error")
	}
	if _, err := c.Get("p"); err == nil || errors.Is(err, ErrNoCap) {
		t.Fatalf("got %v", err)
	}
}
