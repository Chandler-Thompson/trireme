// Package claudecode is the harness adapter for the Claude Code CLI.
package claudecode

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/Chandler-Thompson/trireme/internal/harness"
)

// Runner executes a command in dir and returns its standard output and error.
// It exists so tests can stand in for the real CLI.
type Runner func(ctx context.Context, dir, name string, args []string) (stdout, stderr []byte, err error)

// Adapter drives `claude -p` one turn at a time, resuming the seat's session each time.
type Adapter struct {
	Binary string
	Run    Runner
	NewID  func() (string, error)
}

// usageLimit matches the CLI's report that a usage window or plan limit is spent.
var usageLimit = regexp.MustCompile(`(?i)(hit your (session|usage|weekly|daily) limit|usage limit reached|limit (will )?resets?)`)

// Name identifies the harness in the journal.
func (a *Adapter) Name() string { return "claude-code" }

// Turn sends prompt to the seat's session and parses the CLI's JSON result.
func (a *Adapter) Turn(ctx context.Context, spec harness.SessionSpec, prompt string) (harness.Reply, error) {
	if prompt == "" {
		return harness.Reply{}, fmt.Errorf("claudecode: empty prompt")
	}
	args, sessionID, err := a.args(spec, prompt)
	if err != nil {
		return harness.Reply{}, err
	}
	stdout, stderr, runErr := a.Run(ctx, spec.WorkDir, a.Binary, args)
	reply, parseErr := parseResult(stdout)
	if parseErr == nil {
		if reply.SessionID == "" {
			reply.SessionID = sessionID
		}
		return reply, nil
	}
	// Only a failed turn is checked for the limit message, so a seat that merely
	// talks about usage limits in a successful reply never pauses the fleet.
	if usageLimit.Match(stdout) || usageLimit.Match(stderr) {
		return harness.Reply{SessionID: sessionID, UsageLimited: true}, nil
	}
	return harness.Reply{}, fmt.Errorf("claudecode: run failed (%v; stderr %q): %w",
		runErr, truncate(stderr), parseErr)
}

// Compact runs the CLI's own compaction command against the session.
func (a *Adapter) Compact(ctx context.Context, spec harness.SessionSpec) (harness.Reply, error) {
	if spec.SessionID == "" {
		return harness.Reply{}, fmt.Errorf("claudecode: cannot compact a session that has not started")
	}
	return a.Turn(ctx, spec, "/compact")
}

func (a *Adapter) args(spec harness.SessionSpec, prompt string) ([]string, string, error) {
	args := []string{"-p", prompt, "--output-format", "json"}
	sessionID := spec.SessionID
	if sessionID == "" {
		id, err := a.NewID()
		if err != nil {
			return nil, "", fmt.Errorf("claudecode: new session id: %w", err)
		}
		sessionID = id
		args = append(args, "--session-id", sessionID)
	} else {
		args = append(args, "--resume", sessionID)
	}
	args = appendIf(args, "--model", spec.Model)
	args = appendIf(args, "--agent", spec.Agent)
	args = appendIf(args, "--append-system-prompt", spec.SystemPrompt)
	args = appendIf(args, "--permission-mode", spec.PermissionMode)
	if len(spec.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(spec.AllowedTools, ","))
	}
	return args, sessionID, nil
}

func appendIf(args []string, flag, value string) []string {
	if value == "" {
		return args
	}
	return append(args, flag, value)
}

type usage struct {
	Input         int64 `json:"input_tokens"`
	CacheCreation int64 `json:"cache_creation_input_tokens"`
	CacheRead     int64 `json:"cache_read_input_tokens"`
	Output        int64 `json:"output_tokens"`
}

type result struct {
	Result     string                     `json:"result"`
	SessionID  string                     `json:"session_id"`
	IsError    bool                       `json:"is_error"`
	CostUSD    float64                    `json:"total_cost_usd"`
	ModelUsage map[string]json.RawMessage `json:"modelUsage"`
	Usage      struct {
		usage
		Iterations []usage `json:"iterations"`
	} `json:"usage"`
}

func parseResult(stdout []byte) (harness.Reply, error) {
	var r result
	if err := json.Unmarshal(stdout, &r); err != nil {
		return harness.Reply{}, fmt.Errorf("decode result: %w", err)
	}
	if r.IsError {
		return harness.Reply{}, fmt.Errorf("harness reported an error: %s", truncate([]byte(r.Result)))
	}
	u := r.Usage.usage
	spend := harness.Spend{
		Model:         primaryModel(r.ModelUsage),
		TokensIn:      u.Input + u.CacheCreation + u.CacheRead,
		TokensInFresh: u.Input + u.CacheCreation,
		TokensOut:     u.Output,
		ContextTokens: lastContext(r.Usage.Iterations, u),
		CostUSD:       r.CostUSD,
	}
	return harness.Reply{Text: r.Result, SessionID: r.SessionID, Spend: spend}, nil
}

// lastContext is the context size seen by the turn's final model call.
func lastContext(iterations []usage, total usage) int64 {
	last := total
	if len(iterations) > 0 {
		last = iterations[len(iterations)-1]
	}
	return last.Input + last.CacheCreation + last.CacheRead + last.Output
}

// primaryModel picks the model that produced the most output in the turn.
func primaryModel(models map[string]json.RawMessage) string {
	best, bestOut := "", int64(-1)
	for name, raw := range models {
		var m struct {
			OutputTokens int64 `json:"outputTokens"`
		}
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		if m.OutputTokens > bestOut || (m.OutputTokens == bestOut && name < best) {
			best, bestOut = name, m.OutputTokens
		}
	}
	return best
}

func truncate(b []byte) string {
	const max = 300
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "…"
}

// NewUUID returns a random version-4 UUID, the form the CLI requires for session ids.
func NewUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
