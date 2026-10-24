package claudecode

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Chandler-Thompson/trireme/internal/harness"
)

const okJSON = `{"type":"result","result":"pong","session_id":"sid-1","is_error":false,
"total_cost_usd":0.25,
"usage":{"input_tokens":2,"cache_creation_input_tokens":100,"cache_read_input_tokens":50,"output_tokens":4,
 "iterations":[{"input_tokens":1,"cache_creation_input_tokens":10,"cache_read_input_tokens":5,"output_tokens":1},
               {"input_tokens":1,"cache_creation_input_tokens":90,"cache_read_input_tokens":45,"output_tokens":3}]},
"modelUsage":{"small":{"outputTokens":1},"big":{"outputTokens":3},"broken":"x"}}`

type call struct {
	dir  string
	args []string
}

func fake(stdout, stderr string, err error, calls *[]call) Runner {
	return func(_ context.Context, dir, _ string, args []string) ([]byte, []byte, error) {
		*calls = append(*calls, call{dir, args})
		return []byte(stdout), []byte(stderr), err
	}
}

func adapter(r Runner) *Adapter {
	return &Adapter{Binary: "claude", Run: r, NewID: func() (string, error) { return "new-id", nil }}
}

func flagValue(args []string, flag string) string {
	i := slices.Index(args, flag)
	if i < 0 || i+1 >= len(args) {
		return ""
	}
	return args[i+1]
}

func TestTurn_EmptyPrompt_ReturnsError(t *testing.T) {
	var calls []call
	if _, err := adapter(fake(okJSON, "", nil, &calls)).Turn(context.Background(), harness.SessionSpec{}, ""); err == nil {
		t.Fatal("expected an error")
	}
	if len(calls) != 0 {
		t.Fatal("CLI must not run for an empty prompt")
	}
}

func TestTurn_NewSession_PassesSessionIDAndAllFlags(t *testing.T) {
	var calls []call
	spec := harness.SessionSpec{WorkDir: "/w", Model: "opus", Agent: "xo-agent", SystemPrompt: "card",
		AllowedTools: []string{"Read", "Bash(git:*)"}, PermissionMode: "acceptEdits"}
	if _, err := adapter(fake(okJSON, "", nil, &calls)).Turn(context.Background(), spec, "hi"); err != nil {
		t.Fatal(err)
	}
	args := calls[0].args
	want := map[string]string{"-p": "hi", "--output-format": "json", "--session-id": "new-id", "--model": "opus",
		"--agent": "xo-agent", "--append-system-prompt": "card", "--permission-mode": "acceptEdits",
		"--allowedTools": "Read,Bash(git:*)"}
	for flag, value := range want {
		if got := flagValue(args, flag); got != value {
			t.Errorf("%s = %q, want %q", flag, got, value)
		}
	}
	if calls[0].dir != "/w" || slices.Contains(args, "--resume") {
		t.Fatalf("unexpected call %+v", calls[0])
	}
}

func TestTurn_ExistingSession_ResumesWithoutOptionalFlags(t *testing.T) {
	var calls []call
	if _, err := adapter(fake(okJSON, "", nil, &calls)).Turn(context.Background(), harness.SessionSpec{SessionID: "old"}, "hi"); err != nil {
		t.Fatal(err)
	}
	args := calls[0].args
	if flagValue(args, "--resume") != "old" {
		t.Fatalf("expected --resume old in %v", args)
	}
	for _, flag := range []string{"--session-id", "--model", "--agent", "--append-system-prompt", "--permission-mode", "--allowedTools"} {
		if slices.Contains(args, flag) {
			t.Errorf("unexpected %s in %v", flag, args)
		}
	}
}

func TestTurn_NewIDFails_ReturnsError(t *testing.T) {
	var calls []call
	a := adapter(fake(okJSON, "", nil, &calls))
	a.NewID = func() (string, error) { return "", errors.New("no entropy") }
	if _, err := a.Turn(context.Background(), harness.SessionSpec{}, "hi"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestTurn_SuccessfulResult_ParsesTextSessionAndSpend(t *testing.T) {
	var calls []call
	got, err := adapter(fake(okJSON, "", nil, &calls)).Turn(context.Background(), harness.SessionSpec{}, "hi")
	if err != nil {
		t.Fatal(err)
	}
	want := harness.Spend{Model: "big", TokensIn: 152, TokensInFresh: 102, TokensOut: 4, ContextTokens: 139, CostUSD: 0.25}
	if got.Text != "pong" || got.SessionID != "sid-1" || got.UsageLimited || got.Spend != want {
		t.Fatalf("got %+v, want spend %+v", got, want)
	}
}

func TestTurn_ResultWithoutSessionOrIterations_FallsBackToSpecAndTotals(t *testing.T) {
	var calls []call
	out := `{"result":"ok","usage":{"input_tokens":1,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":4}}`
	got, err := adapter(fake(out, "", nil, &calls)).Turn(context.Background(), harness.SessionSpec{SessionID: "keep"}, "hi")
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionID != "keep" || got.Spend.ContextTokens != 10 || got.Spend.Model != "" {
		t.Fatalf("unexpected reply %+v", got)
	}
}

func TestTurn_ErrorResultWithLimitMessage_ReportsUsageLimited(t *testing.T) {
	var calls []call
	out := `{"is_error":true,"result":"You've hit your session limit · resets 7am (America/New_York)"}`
	got, err := adapter(fake(out, "", errors.New("exit 1"), &calls)).Turn(context.Background(), harness.SessionSpec{}, "hi")
	if err != nil || !got.UsageLimited || got.SessionID != "new-id" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestTurn_LimitMessageOnlyOnStderr_ReportsUsageLimited(t *testing.T) {
	var calls []call
	got, err := adapter(fake("", "Claude usage limit reached", errors.New("exit 1"), &calls)).Turn(context.Background(), harness.SessionSpec{}, "hi")
	if err != nil || !got.UsageLimited {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestTurn_SuccessfulReplyMentioningLimits_DoesNotPause(t *testing.T) {
	var calls []call
	out := `{"result":"I added detection for 'usage limit reached' and 'hit your session limit'."}`
	got, err := adapter(fake(out, "", nil, &calls)).Turn(context.Background(), harness.SessionSpec{}, "hi")
	if err != nil || got.UsageLimited {
		t.Fatalf("a successful reply must never pause the fleet: %+v, %v", got, err)
	}
}

func TestTurn_ErrorResultWithoutLimit_ReturnsError(t *testing.T) {
	var calls []call
	out := `{"is_error":true,"result":"` + strings.Repeat("x", 400) + `"}`
	_, err := adapter(fake(out, "", errors.New("exit 1"), &calls)).Turn(context.Background(), harness.SessionSpec{}, "hi")
	if err == nil || !strings.Contains(err.Error(), "…") {
		t.Fatalf("expected a truncated error, got %v", err)
	}
}

func TestTurn_UnparsableOutput_ReturnsError(t *testing.T) {
	var calls []call
	if _, err := adapter(fake("not json", "boom", errors.New("exit 2"), &calls)).Turn(context.Background(), harness.SessionSpec{}, "hi"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestCompact_NoSession_ReturnsError(t *testing.T) {
	var calls []call
	if _, err := adapter(fake(okJSON, "", nil, &calls)).Compact(context.Background(), harness.SessionSpec{}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestCompact_ExistingSession_SendsCompactCommand(t *testing.T) {
	var calls []call
	if _, err := adapter(fake(okJSON, "", nil, &calls)).Compact(context.Background(), harness.SessionSpec{SessionID: "s"}); err != nil {
		t.Fatal(err)
	}
	if flagValue(calls[0].args, "-p") != "/compact" {
		t.Fatalf("expected /compact, got %v", calls[0].args)
	}
}

func TestPrimaryModel_TiedOutput_PicksAlphabeticallyFirst(t *testing.T) {
	got := primaryModel(map[string]json.RawMessage{
		"b": json.RawMessage(`{"outputTokens":2}`),
		"a": json.RawMessage(`{"outputTokens":2}`),
	})
	if got != "a" {
		t.Fatalf("got %q", got)
	}
}

func TestName_Always_ReturnsClaudeCode(t *testing.T) {
	if (&Adapter{}).Name() != "claude-code" {
		t.Fatal("wrong name")
	}
}

func TestNewUUID_Called_ReturnsVersion4Format(t *testing.T) {
	id, err := NewUUID()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(id) {
		t.Fatalf("bad uuid %q", id)
	}
}

func TestExecRunner_RealCommand_ReturnsOutputAndExitError(t *testing.T) {
	out, errOut, err := ExecRunner(context.Background(), t.TempDir(), "sh", []string{"-c", "echo out; echo err >&2; exit 3"})
	if string(out) != "out\n" || string(errOut) != "err\n" || err == nil {
		t.Fatalf("got %q %q %v", out, errOut, err)
	}
}
