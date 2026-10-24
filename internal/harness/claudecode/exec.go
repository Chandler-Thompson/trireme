package claudecode

import (
	"bytes"
	"context"
	"os/exec"
)

// ExecRunner runs the real CLI as a child process.
func ExecRunner(ctx context.Context, dir, name string, args []string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}
