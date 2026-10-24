// Package harness defines the one interface every harness adapter implements
// (doctrine module "seats", the harness adapter). No other package may depend on a
// specific harness.
package harness

import "context"

// SessionSpec says how to run one turn of a seat's long-lived session.
type SessionSpec struct {
	// SessionID resumes an existing session; empty starts a new one.
	SessionID string
	WorkDir   string
	Model     string
	// Agent names a harness-side agent definition to load (for example a user's own
	// persistent agent in the XO seat). Empty means none.
	Agent string
	// SystemPrompt is appended to the harness's own system prompt: the seat's card.
	SystemPrompt   string
	AllowedTools   []string
	PermissionMode string
}

// Spend is what one turn cost, as the harness reports it.
type Spend struct {
	Model string `json:"model"`
	// TokensIn counts every input token the turn sent, cached or not.
	TokensIn int64 `json:"tokens_in"`
	// TokensInFresh counts input that was not a cache read.
	TokensInFresh int64 `json:"tokens_in_fresh"`
	TokensOut     int64 `json:"tokens_out"`
	// ContextTokens is the size of the session's context at the end of the turn.
	ContextTokens int64 `json:"context_tokens"`
	// CostUSD is the harness's API-equivalent cost for the turn.
	CostUSD float64 `json:"cost_usd"`
}

// Reply is the outcome of one turn.
type Reply struct {
	Text      string
	SessionID string
	Spend     Spend
	// UsageLimited is true when the harness reported that its usage window is spent.
	// The turn did no work, and the fleet must pause.
	UsageLimited bool
}

// Adapter runs turns of seat sessions under one harness.
type Adapter interface {
	Name() string
	Turn(ctx context.Context, spec SessionSpec, prompt string) (Reply, error)
	// Compact asks the harness to compact the session's context.
	Compact(ctx context.Context, spec SessionSpec) (Reply, error)
}
