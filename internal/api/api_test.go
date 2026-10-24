package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Chandler-Thompson/trireme/internal/blockers"
	"github.com/Chandler-Thompson/trireme/internal/budget"
	"github.com/Chandler-Thompson/trireme/internal/fleet"
	"github.com/Chandler-Thompson/trireme/internal/harness"
	"github.com/Chandler-Thompson/trireme/internal/inbox"
	"github.com/Chandler-Thompson/trireme/internal/journal"
	"github.com/Chandler-Thompson/trireme/internal/testutil"
	"github.com/Chandler-Thompson/trireme/internal/xo"
)

type echoSeat struct{}

func (echoSeat) Turn(_ context.Context, prompt, _ string) (harness.Reply, error) {
	return harness.Reply{Text: "heard"}, nil
}

type rig struct {
	env *testutil.Env
	srv *Server
	h   http.Handler
}

func newRig(t *testing.T, token string) *rig {
	env := testutil.New(t)
	caps := budget.New(env.DB, env.Journal)
	if err := caps.Seed("p", 10); err != nil {
		t.Fatal(err)
	}
	srv := &Server{
		Caps:     caps,
		Journal:  env.Journal,
		Fleets:   fleet.New(env.DB, env.Journal),
		Blockers: blockers.New(env.DB, env.Journal, nil, env.Clock.Now, env.Log),
		Inbox:    inbox.New(env.DB, env.Journal, env.Clock.Now),
		Chat:     xo.New(echoSeat{}, env.DB, env.Clock.Now, "test", env.Log),
		Statuses: func() map[string]string { return map[string]string{"p/a": "IDLE"} },
		Projects: []string{"p", "q"},
		Token:    token,
		Web:      fstest.MapFS{"index.html": {Data: []byte("<title>Trireme</title>")}},
		Log:      env.Log,
	}
	return &rig{env: env, srv: srv, h: srv.Handler()}
}

func (g *rig) do(t *testing.T, method, path, body, token string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	g.h.ServeHTTP(rec, req)
	var out map[string]any
	if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("bad json %q: %v", rec.Body.String(), err)
		}
	}
	return rec.Code, out
}

func TestAuth_TokenSet_RejectsMissingOrWrongToken(t *testing.T) {
	g := newRig(t, "secret")
	if code, _ := g.do(t, "GET", "/api/fleets", "", ""); code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", code)
	}
	if code, _ := g.do(t, "GET", "/api/fleets", "", "wrong"); code != http.StatusUnauthorized {
		t.Fatalf("wrong token: %d", code)
	}
	if code, _ := g.do(t, "GET", "/api/fleets", "", "secret"); code != http.StatusOK {
		t.Fatalf("right token: %d", code)
	}
	if code, _ := g.do(t, "GET", "/", "", ""); code != http.StatusOK {
		t.Fatalf("the page itself must load without a token: %d", code)
	}
}

func TestFleets_StartPauseResume_ReportsStateAndSpend(t *testing.T) {
	g := newRig(t, "")
	if code, out := g.do(t, "POST", "/api/fleets/p/start", "", ""); code != 200 || out["state"] != "running" {
		t.Fatalf("start %d %v", code, out)
	}
	if _, err := g.env.Journal.Append(journal.Event{ProjectID: "p", Seat: "a", Type: "model.call", Payload: map[string]any{"cost_usd": 1.5}}); err != nil {
		t.Fatal(err)
	}
	_, out := g.do(t, "GET", "/api/fleets", "", "")
	f := out["fleets"].([]any)[0].(map[string]any)
	if f["spent_usd"] != 1.5 || out["seats"].(map[string]any)["p/a"] != "IDLE" {
		t.Fatalf("fleets %v", out)
	}
	if q := out["fleets"].([]any)[1].(map[string]any); q["project_id"] != "q" || q["state"] != "not_started" {
		t.Fatalf("unstarted project missing: %v", out)
	}
	if code, out := g.do(t, "POST", "/api/fleets/p/pause", "", ""); code != 200 || out["reason"] != "user" {
		t.Fatalf("pause %d %v", code, out)
	}
	if code, _ := g.do(t, "POST", "/api/fleets/p/resume", "", ""); code != 200 {
		t.Fatalf("resume %d", code)
	}
}

func TestFleets_BadRequests_ReturnClientErrors(t *testing.T) {
	g := newRig(t, "")
	cases := []struct {
		path string
		code int
	}{
		{"/api/fleets/nope/pause", http.StatusNotFound},
		{"/api/fleets/p/explode", http.StatusNotFound},
	}
	for _, c := range cases {
		if code, _ := g.do(t, "POST", c.path, "", ""); code != c.code {
			t.Errorf("%s: %d, want %d", c.path, code, c.code)
		}
	}
	g.do(t, "POST", "/api/fleets/p/start", "", "")
	if code, _ := g.do(t, "POST", "/api/fleets/p/archive", "", ""); code != http.StatusConflict {
		t.Errorf("archive running: %d", code)
	}
	if code, _ := g.do(t, "POST", "/api/fleets/p/stop", "", ""); code != 200 {
		t.Errorf("stop: %d", code)
	}
	if code, _ := g.do(t, "POST", "/api/fleets/p/archive", "", ""); code != 200 {
		t.Errorf("archive: %d", code)
	}
}

func TestBlockers_OpenListAnswer_RoundTrips(t *testing.T) {
	g := newRig(t, "")
	code, out := g.do(t, "POST", "/api/blockers", `{"project_id":"p","seat":"technical-producer","summary":"Need QA","options":["Accept","Decline"],"kind":"fleet_paused"}`, "")
	if code != 200 || out["kind"] != "general" {
		t.Fatalf("open %d %v (seats must not open blockers the app acts on)", code, out)
	}
	_, out = g.do(t, "GET", "/api/blockers?open=1", "", "")
	if len(out["blockers"].([]any)) != 1 {
		t.Fatalf("list %v", out)
	}
	if code, _ := g.do(t, "POST", "/api/blockers/1/answer", `{"choice":"Accept","text":"yes"}`, ""); code != 200 {
		t.Fatalf("answer %d", code)
	}
	_, out = g.do(t, "GET", "/api/blockers?open=1", "", "")
	if len(out["blockers"].([]any)) != 0 {
		t.Fatalf("still open %v", out)
	}
}

func TestBlockers_ResumeAnswerOnPauseBlocker_ResumesFleet(t *testing.T) {
	g := newRig(t, "")
	g.do(t, "POST", "/api/fleets/p/start", "", "")
	if _, err := g.srv.Fleets.Pause("p", fleet.ReasonUsageWindow); err != nil {
		t.Fatal(err)
	}
	b, err := g.srv.Blockers.Open(blockers.Blocker{ProjectID: "p", Seat: "system", Kind: blockers.KindFleetPaused, Summary: "paused", Options: []string{"Resume"}})
	if err != nil {
		t.Fatal(err)
	}
	if code, out := g.do(t, "POST", "/api/blockers/"+itoa(b.ID)+"/answer", `{"choice":"Resume"}`, ""); code != 200 {
		t.Fatalf("answer %d %v", code, out)
	}
	if st, _ := g.srv.Fleets.Get("p"); st.State != fleet.Running {
		t.Fatalf("not resumed: %+v", st)
	}
}

func TestBlockers_ResumeWhenFleetNotPaused_ReturnsConflict(t *testing.T) {
	g := newRig(t, "")
	b, _ := g.srv.Blockers.Open(blockers.Blocker{ProjectID: "p", Seat: "system", Kind: blockers.KindFleetPaused, Summary: "paused", Options: []string{"Resume"}})
	if code, _ := g.do(t, "POST", "/api/blockers/"+itoa(b.ID)+"/answer", `{"choice":"Resume"}`, ""); code != http.StatusConflict {
		t.Fatalf("got %d", code)
	}
}

func TestBlockers_BadRequests_ReturnClientErrors(t *testing.T) {
	g := newRig(t, "")
	cases := []struct{ method, path, body string }{
		{"POST", "/api/blockers", `{"summary":"no project"}`},
		{"POST", "/api/blockers", `{"unknown_field":1}`},
		{"POST", "/api/blockers", `not json`},
		{"POST", "/api/blockers/x/answer", `{}`},
		{"POST", "/api/blockers/9/answer", `{"text":"x"}`},
		{"POST", "/api/blockers/9/answer", `{`},
	}
	for _, c := range cases {
		if code, _ := g.do(t, c.method, c.path, c.body, ""); code != http.StatusBadRequest {
			t.Errorf("%s %s %s: %d", c.method, c.path, c.body, code)
		}
	}
}

func TestJournalAndInbox_PostThenRead_ShowsEvent(t *testing.T) {
	g := newRig(t, "")
	if code, _ := g.do(t, "POST", "/api/inbox", `{"project_id":"p","to_seat":"technical-lead","from_seat":"xo","kind":"directive","body":"hi"}`, ""); code != 200 {
		t.Fatalf("inbox %d", code)
	}
	if code, _ := g.do(t, "POST", "/api/inbox", `{"kind":"nope"}`, ""); code != 400 {
		t.Fatalf("bad inbox %d", code)
	}
	if code, _ := g.do(t, "POST", "/api/inbox", `{`, ""); code != 400 {
		t.Fatalf("bad body %d", code)
	}
	_, out := g.do(t, "GET", "/api/journal?project=p&after=0&limit=5000", "", "")
	events := out["events"].([]any)
	if len(events) != 1 || events[0].(map[string]any)["type"] != "directive.posted" {
		t.Fatalf("journal %v", out)
	}
}

func TestChat_SayThenRead_ShowsBothLines(t *testing.T) {
	g := newRig(t, "")
	if code, _ := g.do(t, "POST", "/api/chat", `{"text":""}`, ""); code != 400 {
		t.Fatalf("empty %d", code)
	}
	if code, _ := g.do(t, "POST", "/api/chat", `{`, ""); code != 400 {
		t.Fatalf("bad body %d", code)
	}
	if code, _ := g.do(t, "POST", "/api/chat", `{"text":"hello"}`, ""); code != 200 {
		t.Fatalf("say %d", code)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, out := g.do(t, "GET", "/api/chat?after=0", "", "")
		if lines := out["lines"].([]any); len(lines) == 2 {
			if lines[1].(map[string]any)["text"] != "heard" {
				t.Fatalf("lines %v", lines)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("reply never arrived")
}

func TestServer_DatabaseGone_ReturnsServerErrors(t *testing.T) {
	g := newRig(t, "")
	g.do(t, "POST", "/api/fleets/p/start", "", "")
	g.env.DB.Close()
	for _, path := range []string{"/api/fleets", "/api/blockers", "/api/journal", "/api/chat"} {
		if code, _ := g.do(t, "GET", path, "", ""); code != http.StatusInternalServerError {
			t.Errorf("%s: %d", path, code)
		}
	}
}

func TestFleets_SpendUnreadable_ReturnsServerError(t *testing.T) {
	g := newRig(t, "")
	g.do(t, "POST", "/api/fleets/p/start", "", "")
	if _, err := g.env.DB.Exec(`ALTER TABLE events RENAME TO events_gone; CREATE TABLE events (id INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if code, _ := g.do(t, "GET", "/api/fleets", "", ""); code != http.StatusInternalServerError {
		t.Fatalf("got %d", code)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestCap_BlockerRaise_RaisesCapAndResumes(t *testing.T) {
	g := newRig(t, "")
	g.do(t, "POST", "/api/fleets/p/start", "", "")
	if _, err := g.env.Journal.Append(journal.Event{ProjectID: "p", Seat: "a", Type: "model.call", Payload: map[string]any{"cost_usd": 10}}); err != nil {
		t.Fatal(err)
	}
	if _, err := g.srv.Fleets.Pause("p", fleet.ReasonProjectCap); err != nil {
		t.Fatal(err)
	}
	if code, out := g.do(t, "POST", "/api/fleets/p/resume", "", ""); code != http.StatusConflict || !strings.Contains(out["error"].(string), "raise the cap") {
		t.Fatalf("resume over cap: %d %v", code, out)
	}
	b, _ := g.srv.Blockers.Open(blockers.Blocker{ProjectID: "p", Seat: "system", Kind: blockers.KindCapReached, Summary: "cap", Options: []string{blockers.ChoiceRaiseCap}})
	if code, _ := g.do(t, "POST", "/api/blockers/"+itoa(b.ID)+"/answer", `{"choice":"Raise cap and resume","text":"ten"}`, ""); code != http.StatusConflict {
		t.Fatalf("non-number: %d", code)
	}
	b2, _ := g.srv.Blockers.Open(blockers.Blocker{ProjectID: "p", Seat: "system", Kind: blockers.KindCapReached, Summary: "cap", Options: []string{blockers.ChoiceRaiseCap}})
	if code, out := g.do(t, "POST", "/api/blockers/"+itoa(b2.ID)+"/answer", `{"choice":"Raise cap and resume","text":"$25"}`, ""); code != 200 {
		t.Fatalf("raise: %d %v", code, out)
	}
	_, out := g.do(t, "GET", "/api/fleets", "", "")
	f := out["fleets"].([]any)[0].(map[string]any)
	if f["state"] != "running" || f["cap_usd"] != 25.0 {
		t.Fatalf("fleet %v", f)
	}
}

func TestCap_RaiseEndpoint_ValidatesAmount(t *testing.T) {
	g := newRig(t, "")
	if code, _ := g.do(t, "POST", "/api/projects/p/cap", `{"cap_usd":5}`, ""); code != http.StatusBadRequest {
		t.Fatalf("lower: %d", code)
	}
	if code, _ := g.do(t, "POST", "/api/projects/p/cap", `{`, ""); code != http.StatusBadRequest {
		t.Fatalf("bad body: %d", code)
	}
	if code, out := g.do(t, "POST", "/api/projects/p/cap", `{"cap_usd":50}`, ""); code != 200 || out["cap_usd"] != 50.0 {
		t.Fatalf("raise: %d %v", code, out)
	}
}

func TestFleets_CapTableGone_ReturnsServerError(t *testing.T) {
	g := newRig(t, "")
	g.do(t, "POST", "/api/fleets/p/start", "", "")
	if _, err := g.env.DB.Exec(`DROP TABLE caps`); err != nil {
		t.Fatal(err)
	}
	if code, _ := g.do(t, "GET", "/api/fleets", "", ""); code != http.StatusInternalServerError {
		t.Fatalf("list: %d", code)
	}
	if _, err := g.srv.Fleets.Pause("p", fleet.ReasonUser); err != nil {
		t.Fatal(err)
	}
	if code, _ := g.do(t, "POST", "/api/fleets/p/resume", "", ""); code != http.StatusConflict {
		t.Fatalf("resume: %d", code)
	}
}
