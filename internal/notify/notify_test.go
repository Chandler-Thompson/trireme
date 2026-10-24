package notify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Chandler-Thompson/trireme/internal/blockers"
)

func TestNotify_NoCommand_ReturnsError(t *testing.T) {
	if err := (Command{}).Notify(blockers.Blocker{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestNotify_Command_ReceivesBlockerJSONOnStdin(t *testing.T) {
	out := filepath.Join(t.TempDir(), "got.json")
	c := Command{Argv: []string{"sh", "-c", "cat > " + out}, Timeout: 5 * time.Second}
	if err := c.Notify(blockers.Blocker{ID: 7, Summary: "Need a seat"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(got), `"summary":"Need a seat"`) || !strings.Contains(string(got), `"id":7`) {
		t.Fatalf("got %q %v", got, err)
	}
}

func TestNotify_CommandFails_ReturnsItsOutput(t *testing.T) {
	c := Command{Argv: []string{"sh", "-c", "echo topic missing; exit 4"}, Timeout: 5 * time.Second}
	err := c.Notify(blockers.Blocker{})
	if err == nil || !strings.Contains(err.Error(), "topic missing") {
		t.Fatalf("got %v", err)
	}
}

func TestNotify_CommandHangs_IsKilledAtTimeout(t *testing.T) {
	c := Command{Argv: []string{"sleep", "10"}, Timeout: 50 * time.Millisecond}
	start := time.Now()
	if err := c.Notify(blockers.Blocker{}); err == nil || time.Since(start) > 5*time.Second {
		t.Fatalf("err %v after %v", err, time.Since(start))
	}
}

func TestTruncate_LongOutput_IsShortened(t *testing.T) {
	if got := truncate([]byte(strings.Repeat("x", 500))); len(got) > 210 || !strings.HasSuffix(got, "…") {
		t.Fatalf("got %d chars", len(got))
	}
}
