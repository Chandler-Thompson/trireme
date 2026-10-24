// Package api is the HTTP interface. Every action the UI offers is an API call here, so
// seats and the XO can do anything the user can without driving the UI.
package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/Chandler-Thompson/trireme/internal/blockers"
	"github.com/Chandler-Thompson/trireme/internal/budget"
	"github.com/Chandler-Thompson/trireme/internal/fleet"
	"github.com/Chandler-Thompson/trireme/internal/inbox"
	"github.com/Chandler-Thompson/trireme/internal/journal"
	"github.com/Chandler-Thompson/trireme/internal/xo"
)

// Server wires the services to HTTP.
type Server struct {
	Journal  *journal.Journal
	Fleets   *fleet.Registry
	Blockers *blockers.Queue
	Caps     *budget.Caps
	Inbox    *inbox.Inbox
	Chat     *xo.Chat
	Statuses func() map[string]string
	// Projects lists every configured project, so ones whose fleet never started still
	// show, with a way to start them.
	Projects []string
	// Token, when set, is required as a bearer token on every /api call.
	Token string
	Web   fs.FS
	Log   *slog.Logger
}

// Handler returns the routed handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/fleets", s.listFleets)
	mux.HandleFunc("POST /api/fleets/{id}/{action}", s.fleetAction)
	mux.HandleFunc("POST /api/projects/{id}/cap", s.raiseCap)
	mux.HandleFunc("GET /api/blockers", s.listBlockers)
	mux.HandleFunc("POST /api/blockers", s.openBlocker)
	mux.HandleFunc("POST /api/blockers/{id}/answer", s.answerBlocker)
	mux.HandleFunc("GET /api/journal", s.listJournal)
	mux.HandleFunc("POST /api/inbox", s.postInbox)
	mux.HandleFunc("GET /api/chat", s.transcript)
	mux.HandleFunc("POST /api/chat", s.say)
	if s.Web != nil {
		mux.Handle("GET /", http.FileServerFS(s.Web))
	}
	return s.auth(mux)
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Token == "" || !isAPI(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		want := "Bearer " + s.Token
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(want)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isAPI(path string) bool { return len(path) >= 5 && path[:5] == "/api/" }

type fleetView struct {
	fleet.State
	SpentUSD float64 `json:"spent_usd"`
	CapUSD   float64 `json:"cap_usd"`
}

func (s *Server) listFleets(w http.ResponseWriter, _ *http.Request) {
	states, err := s.Fleets.List()
	if err != nil {
		s.fail(w, err, http.StatusInternalServerError)
		return
	}
	views := make([]fleetView, 0, len(states))
	known := map[string]bool{}
	for _, st := range states {
		known[st.ProjectID] = true
		spent, err := s.Journal.Spent(st.ProjectID)
		if err != nil {
			s.fail(w, err, http.StatusInternalServerError)
			return
		}
		usd, err := s.Caps.Get(st.ProjectID)
		if err != nil && !errors.Is(err, budget.ErrNoCap) {
			s.fail(w, err, http.StatusInternalServerError)
			return
		}
		views = append(views, fleetView{State: st, SpentUSD: spent, CapUSD: usd})
	}
	for _, p := range s.Projects {
		if !known[p] {
			views = append(views, fleetView{State: fleet.State{ProjectID: p, State: "not_started"}})
		}
	}
	s.ok(w, map[string]any{"fleets": views, "seats": s.Statuses()})
}

func (s *Server) fleetAction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var err error
	switch r.PathValue("action") {
	case "start":
		err = s.Fleets.Start(id)
	case "pause":
		_, err = s.Fleets.Pause(id, fleet.ReasonUser)
	case "resume":
		err = s.resume(id)
	case "stop":
		err = s.Fleets.Stop(id)
	case "archive":
		err = s.Fleets.Archive(id)
	default:
		s.fail(w, fmt.Errorf("unknown action %q", r.PathValue("action")), http.StatusNotFound)
		return
	}
	if err != nil {
		s.fail(w, err, statusFor(err))
		return
	}
	st, err := s.Fleets.Get(id)
	if err != nil {
		s.fail(w, err, statusFor(err))
		return
	}
	s.ok(w, st)
}

func statusFor(err error) int {
	if errors.Is(err, fleet.ErrUnknown) {
		return http.StatusNotFound
	}
	return http.StatusConflict
}

func (s *Server) listBlockers(w http.ResponseWriter, r *http.Request) {
	list, err := s.Blockers.List(r.URL.Query().Get("open") == "1")
	if err != nil {
		s.fail(w, err, http.StatusInternalServerError)
		return
	}
	s.ok(w, map[string]any{"blockers": nonNil(list)})
}

func (s *Server) openBlocker(w http.ResponseWriter, r *http.Request) {
	var b blockers.Blocker
	if !s.decode(w, r, &b) {
		return
	}
	b.Kind = blockers.KindGeneral // why: only the application opens blockers it acts on
	opened, err := s.Blockers.Open(b)
	if err != nil {
		s.fail(w, err, http.StatusBadRequest)
		return
	}
	s.ok(w, opened)
}

func (s *Server) answerBlocker(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.fail(w, fmt.Errorf("bad blocker id: %w", err), http.StatusBadRequest)
		return
	}
	var body struct{ Choice, Text string }
	if !s.decode(w, r, &body) {
		return
	}
	b, err := s.Blockers.Answer(id, body.Choice, body.Text)
	if err != nil {
		s.fail(w, err, http.StatusBadRequest)
		return
	}
	if err := s.actOn(b); err != nil {
		s.fail(w, err, http.StatusConflict)
		return
	}
	s.ok(w, b)
}

// actOn carries out answers the application understands. Resuming is manual by
// doctrine, and answering a pause blocker is the user doing it.
func (s *Server) actOn(b blockers.Blocker) error {
	switch {
	case b.Kind == blockers.KindFleetPaused && b.Choice == blockers.ChoiceResume:
		return s.resume(b.ProjectID)
	case b.Kind == blockers.KindCapReached && b.Choice == blockers.ChoiceRaiseCap:
		usd, err := strconv.ParseFloat(strings.TrimPrefix(strings.TrimSpace(b.Text), "$"), 64)
		if err != nil {
			return fmt.Errorf("the new cap must be a dollar amount, got %q", b.Text)
		}
		if err := s.Caps.Raise(b.ProjectID, usd); err != nil {
			return err
		}
		return s.resume(b.ProjectID)
	}
	return nil
}

// resume refuses to restart a fleet that is still at its cap, which would only pause it
// again on its next turn.
func (s *Server) resume(projectID string) error {
	over, spent, usd, err := s.Caps.Over(projectID)
	if err != nil && !errors.Is(err, budget.ErrNoCap) {
		return err
	}
	if over {
		return fmt.Errorf("the fleet has spent $%.2f of its $%.2f cap; raise the cap first", spent, usd)
	}
	return s.Fleets.Resume(projectID)
}

func (s *Server) raiseCap(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CapUSD float64 `json:"cap_usd"`
	}
	if !s.decode(w, r, &body) {
		return
	}
	if err := s.Caps.Raise(r.PathValue("id"), body.CapUSD); err != nil {
		s.fail(w, err, http.StatusBadRequest)
		return
	}
	s.ok(w, map[string]any{"project_id": r.PathValue("id"), "cap_usd": body.CapUSD})
}

func (s *Server) listJournal(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	after, _ := strconv.ParseInt(q.Get("after"), 10, 64) // why: absent or bad means from the start
	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil || limit <= 0 || limit > 1000 {
		limit = 200
	}
	events, err := s.Journal.Since(q.Get("project"), after, limit)
	if err != nil {
		s.fail(w, err, http.StatusInternalServerError)
		return
	}
	s.ok(w, map[string]any{"events": nonNil(events)})
}

func (s *Server) postInbox(w http.ResponseWriter, r *http.Request) {
	var it inbox.Item
	if !s.decode(w, r, &it) {
		return
	}
	posted, err := s.Inbox.Post(it)
	if err != nil {
		s.fail(w, err, http.StatusBadRequest)
		return
	}
	s.ok(w, posted)
}

func (s *Server) transcript(w http.ResponseWriter, r *http.Request) {
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64) // why: absent or bad means from the start
	lines, err := s.Chat.Transcript(after)
	if err != nil {
		s.fail(w, err, http.StatusInternalServerError)
		return
	}
	s.ok(w, map[string]any{"lines": nonNil(lines), "busy": s.Chat.Busy()})
}

func (s *Server) say(w http.ResponseWriter, r *http.Request) {
	var body struct{ Text string }
	if !s.decode(w, r, &body) {
		return
	}
	line, err := s.Chat.Say(r.Context(), body.Text)
	if err != nil {
		s.fail(w, err, http.StatusBadRequest)
		return
	}
	s.ok(w, line)
}

func (s *Server) decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		s.fail(w, fmt.Errorf("bad request body: %w", err), http.StatusBadRequest)
		return false
	}
	return true
}

func (s *Server) ok(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.Log.Error("write response failed", "component", "api", "error", err)
	}
}

func (s *Server) fail(w http.ResponseWriter, err error, code int) {
	if code >= 500 {
		s.Log.Error("request failed", "component", "api", "error", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if encErr := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); encErr != nil {
		s.Log.Error("write error response failed", "component", "api", "error", encErr)
	}
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
