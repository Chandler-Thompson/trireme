// Package notify is Trireme's notification hook. An instance plugs in any notifier it
// likes (a phone push, a chat message) as an external command, so Trireme itself never
// knows which service the instance uses.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/Chandler-Thompson/trireme/internal/blockers"
)

// Command runs an external program once per new blocker, with the blocker as JSON on
// standard input.
type Command struct {
	Argv    []string
	Timeout time.Duration
}

// Notify runs the command and returns its failure, including anything it printed.
func (c Command) Notify(b blockers.Blocker) error {
	if len(c.Argv) == 0 {
		return fmt.Errorf("notify: no command configured")
	}
	payload, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("notify: encode blocker: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Argv[0], c.Argv[1:]...)
	cmd.Stdin = bytes.NewReader(payload)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("notify: %s: %w: %s", c.Argv[0], err, truncate(out))
	}
	return nil
}

func truncate(b []byte) string {
	const max = 200
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "…"
}
