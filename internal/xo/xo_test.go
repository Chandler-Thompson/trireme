package xo

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Chandler-Thompson/trireme/internal/harness"
	"github.com/Chandler-Thompson/trireme/internal/seat"
	"github.com/Chandler-Thompson/trireme/internal/testutil"
)

type fakeSeat struct {
	text    string
	err     error
	prompts []string
	gate    chan struct{}
}

func (f *fakeSeat) Turn(_ context.Context, prompt, _ string) (harness.Reply, error) {
	if f.gate != nil {
		<-f.gate
	}
	f.prompts = append(f.prompts, prompt)
	return harness.Reply{Text: f.text}, f.err
}

func waitFor(t *testing.T, c *Chat, n int) []Line {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		lines, err := c.Transcript(0)
		if err != nil {
			t.Fatal(err)
		}
		if len(lines) >= n {
			return lines
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("transcript never reached %d lines", n)
	return nil
}

func TestSay_EmptyText_ReturnsError(t *testing.T) {
	env := testutil.New(t)
	c := New(&fakeSeat{}, env.DB, env.Clock.Now, "desk-1", env.Log)
	if _, err := c.Say(context.Background(), ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestSay_Message_RecordsUserLineThenXOReply(t *testing.T) {
	env := testutil.New(t)
	f := &fakeSeat{text: "All three fleets are idle."}
	c := New(f, env.DB, env.Clock.Now, "server-1", env.Log)
	line, err := c.Say(context.Background(), "status?")
	if err != nil || line.Speaker != SpeakerUser || line.ID != 1 {
		t.Fatalf("line %+v err %v", line, err)
	}
	lines := waitFor(t, c, 2)
	if lines[1].Speaker != SpeakerXO || lines[1].Text != f.text {
		t.Fatalf("lines %+v", lines)
	}
	if !strings.Contains(f.prompts[0], "server on server-1") || !strings.Contains(f.prompts[0], "status?") {
		t.Fatalf("prompt %q", f.prompts[0])
	}
}

func TestSay_UsageLimited_RecordsSystemLine(t *testing.T) {
	env := testutil.New(t)
	c := New(&fakeSeat{err: seat.ErrUsageLimited}, env.DB, env.Clock.Now, "x", env.Log)
	if _, err := c.Say(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	if l := waitFor(t, c, 2)[1]; l.Speaker != SpeakerSystem || !strings.Contains(l.Text, "usage window") {
		t.Fatalf("line %+v", l)
	}
}

func TestSay_TurnFails_RecordsFailure(t *testing.T) {
	env := testutil.New(t)
	c := New(&fakeSeat{err: errors.New("boom")}, env.DB, env.Clock.Now, "x", env.Log)
	if _, err := c.Say(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	if l := waitFor(t, c, 2)[1]; l.Speaker != SpeakerSystem || !strings.Contains(l.Text, "boom") {
		t.Fatalf("line %+v", l)
	}
}

func TestBusy_DuringTurn_ReportsTrueThenFalse(t *testing.T) {
	env := testutil.New(t)
	f := &fakeSeat{text: "ok", gate: make(chan struct{})}
	c := New(f, env.DB, env.Clock.Now, "x", env.Log)
	if _, err := c.Say(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !c.Busy() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !c.Busy() {
		t.Fatal("never busy")
	}
	close(f.gate)
	waitFor(t, c, 2)
	for c.Busy() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if c.Busy() {
		t.Fatal("still busy")
	}
}

func TestTranscript_Broken_ReturnsErrors(t *testing.T) {
	env := testutil.New(t)
	c := New(&fakeSeat{}, env.DB, env.Clock.Now, "x", env.Log)
	if _, err := env.DB.Exec(`INSERT INTO transcript (ts, speaker, text) VALUES ('nope', 'user', 'x')`); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Transcript(0); err == nil {
		t.Fatal("expected parse error")
	}
	if _, err := env.DB.Exec(`DROP TABLE transcript`); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Transcript(0); err == nil {
		t.Fatal("expected query error")
	}
	if _, err := c.Say(context.Background(), "hi"); err == nil {
		t.Fatal("expected write error")
	}
	c.reply(context.Background(), "hi") // logs the write failure; must not panic
}
