// Command trireme runs a Trireme instance: the API, the web UI, the XO chat, and the
// runner that gives seats their turns.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Chandler-Thompson/trireme/internal/api"
	"github.com/Chandler-Thompson/trireme/internal/blockers"
	"github.com/Chandler-Thompson/trireme/internal/budget"
	"github.com/Chandler-Thompson/trireme/internal/config"
	"github.com/Chandler-Thompson/trireme/internal/fleet"
	"github.com/Chandler-Thompson/trireme/internal/harness"
	"github.com/Chandler-Thompson/trireme/internal/harness/claudecode"
	"github.com/Chandler-Thompson/trireme/internal/inbox"
	"github.com/Chandler-Thompson/trireme/internal/journal"
	"github.com/Chandler-Thompson/trireme/internal/notify"
	"github.com/Chandler-Thompson/trireme/internal/runner"
	"github.com/Chandler-Thompson/trireme/internal/seat"
	"github.com/Chandler-Thompson/trireme/internal/store"
	"github.com/Chandler-Thompson/trireme/internal/xo"
	"github.com/Chandler-Thompson/trireme/web"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("trireme stopped", "component", "main", "error", err)
		os.Exit(1)
	}
}

// services is everything the instance runs, built once at startup.
type services struct {
	cfg     config.Instance
	journal *journal.Journal
	fleets  *fleet.Registry
	queue   *blockers.Queue
	caps    *budget.Caps
	inbox   *inbox.Inbox
	runner  *runner.Runner
	chat    *xo.Chat
	token   string
}

func run(log *slog.Logger) error {
	cfgPath := flag.String("config", "trireme.json", "path to the instance settings file")
	flag.Parse()
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	db, err := store.Open(filepath.Join(cfg.DataDir, "trireme.db"))
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }() // why: process exit follows; nothing to recover
	svc, err := build(cfg, db, log)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go svc.runner.Loop(ctx, 5*time.Second)
	return serve(ctx, svc, log)
}

func build(cfg config.Instance, db *sql.DB, log *slog.Logger) (*services, error) {
	token, err := loadToken(cfg)
	if err != nil {
		return nil, err
	}
	now := time.Now
	j := journal.New(db, now)
	var notifier blockers.Notifier
	if len(cfg.NotifyCommand) > 0 {
		notifier = notify.Command{Argv: cfg.NotifyCommand, Timeout: 30 * time.Second}
	}
	svc := &services{cfg: cfg, journal: j, token: token, fleets: fleet.New(db, j),
		queue: blockers.New(db, j, notifier, now, log), inbox: inbox.New(db, j, now), caps: budget.New(db, j)}
	for _, p := range cfg.Projects {
		if err := svc.caps.Seed(p.ID, p.CapUSD); err != nil {
			return nil, err
		}
	}
	adapter := &claudecode.Adapter{Binary: cfg.ClaudeBinary, Run: claudecode.ExecRunner, NewID: claudecode.NewUUID}
	deps := seat.Deps{Adapter: adapter, Journal: j, Fleets: svc.fleets, Blockers: svc.queue, Caps: svc.caps,
		Sessions: seat.NewSessions(db), Log: log}
	slots, err := buildSlots(cfg, deps)
	if err != nil {
		return nil, err
	}
	svc.runner = runner.New(slots, svc.inbox, svc.fleets, now, time.Minute, log)
	xoSeat, err := buildXO(cfg, deps)
	if err != nil {
		return nil, err
	}
	svc.chat = xo.New(xoSeat, db, now, cfg.BodyLabel, log)
	setChildEnv(cfg, token)
	return svc, nil
}

func buildSlots(cfg config.Instance, deps seat.Deps) ([]*runner.Slot, error) {
	var slots []*runner.Slot
	for _, p := range cfg.Projects {
		for _, s := range p.Seats {
			card, err := cfg.Card(s.ID)
			if err != nil {
				return nil, err
			}
			st := seat.New(seat.Config{ProjectID: p.ID, Seat: s.ID, DoctrineVersion: cfg.DoctrineVersion,
				Card: card + protocol(cfg.DoctrineDir), CompactAt: p.CompactAt,
				Spec: harness.SessionSpec{WorkDir: p.Repo, Model: s.Model, AllowedTools: s.AllowedTools,
					PermissionMode: s.PermissionMode}}, deps)
			slots = append(slots, &runner.Slot{ProjectID: p.ID, SeatID: s.ID, Seat: st})
		}
	}
	return slots, nil
}

func buildXO(cfg config.Instance, deps seat.Deps) (*seat.Seat, error) {
	card, err := cfg.XOCard()
	if err != nil {
		return nil, err
	}
	// The XO belongs to no fleet: it answers even while every fleet is paused.
	deps.Fleets, deps.Blockers, deps.Caps = nil, nil, nil
	workdir := cfg.XO.WorkDir
	if workdir == "" {
		workdir = cfg.DataDir
	}
	return seat.New(seat.Config{ProjectID: "instance", Seat: "xo", DoctrineVersion: cfg.DoctrineVersion,
		Card: card + cfg.ProjectsBrief() + protocol(cfg.DoctrineDir), Spec: harness.SessionSpec{WorkDir: workdir, Model: cfg.XO.Model, Agent: cfg.XO.Agent,
			AllowedTools: cfg.XO.AllowedTools, PermissionMode: cfg.XO.PermissionMode}}, deps), nil
}

// protocol is appended to every briefing so the seat knows where the doctrine is and how
// to reach the shared state.
func protocol(doctrineDir string) string {
	return fmt.Sprintf(`

## Working inside Trireme

The doctrine this fleet runs under is at %s. When your briefing names a module, a
setting or the glossary, read it there.

You reach the shared state only through the trireme-cli command, which is already
configured: "trireme-cli journal", "trireme-cli blockers", "trireme-cli blocker open",
"trireme-cli consult", "trireme-cli answer", "trireme-cli directive" and
"trireme-cli fleets". Run "trireme-cli help" for the flags. Never message another seat
any other way.`, doctrineDir)
}

func setChildEnv(cfg config.Instance, token string) {
	// Seats and the XO inherit these, so trireme-cli inside them reaches this server.
	host := cfg.Listen
	if strings.HasPrefix(host, ":") {
		host = "127.0.0.1" + host
	}
	_ = os.Setenv("TRIREME_URL", "http://"+host) // why: Setenv only fails on invalid keys
	_ = os.Setenv("TRIREME_TOKEN", token)        // why: same
}

func loadToken(cfg config.Instance) (string, error) {
	if cfg.TokenFile == "" {
		if !isLoopback(cfg.Listen) {
			return "", fmt.Errorf("listening on %s needs a token_file", cfg.Listen)
		}
		return "", nil
	}
	raw, err := os.ReadFile(cfg.TokenFile)
	if err != nil {
		return "", fmt.Errorf("read token file: %w", err)
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return "", fmt.Errorf("token file %s is empty", cfg.TokenFile)
	}
	return token, nil
}

func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return host == "localhost" || (ip != nil && ip.IsLoopback())
}

func serve(ctx context.Context, svc *services, log *slog.Logger) error {
	ui, err := web.FS()
	if err != nil {
		return err
	}
	a := &api.Server{Journal: svc.journal, Fleets: svc.fleets, Blockers: svc.queue, Caps: svc.caps, Inbox: svc.inbox,
		Chat: svc.chat, Statuses: svc.runner.Statuses, Projects: projectIDs(svc.cfg), Token: svc.token, Web: ui, Log: log}
	srv := &http.Server{Addr: svc.cfg.Listen, Handler: a.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			log.Error("shutdown failed", "component", "main", "error", err)
		}
	}()
	log.Info("listening", "component", "main", "event", "server.started", "addr", svc.cfg.Listen,
		"message", "Trireme is up at http://"+svc.cfg.Listen)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func projectIDs(cfg config.Instance) []string {
	ids := make([]string, 0, len(cfg.Projects))
	for _, p := range cfg.Projects {
		ids = append(ids, p.ID)
	}
	return ids
}
