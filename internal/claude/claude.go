// Package claude wraps the Anthropic claude CLI, building and executing it as
// a subprocess on behalf of HTTP requests.
package claude

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runner builds and executes claude CLI subprocesses.
type Runner struct {
	// Path is the configured path to (or PATH-resolvable name of) the binary.
	Path string
	// DefaultModel is used when a request does not specify a model.
	DefaultModel string
}

// New returns a Runner for the given claude binary path and default model.
func New(path, defaultModel string) *Runner {
	return &Runner{Path: path, DefaultModel: defaultModel}
}

// Available reports whether the claude binary can be located on disk or PATH.
// It is cheap enough to call on every /health request.
func (r *Runner) Available() error {
	if _, err := exec.LookPath(r.Path); err != nil {
		return fmt.Errorf("claude CLI not found at %q: %w", r.Path, err)
	}
	return nil
}

// Verify confirms the claude CLI is installed and runnable by executing
// "<path> --version". It returns an error if the binary cannot be located or
// the version command fails.
func (r *Runner) Verify(ctx context.Context) error {
	if err := r.Available(); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, r.Path, "--version")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%q --version failed: %v: %s", r.Path, err, msg)
		}
		return fmt.Errorf("%q --version failed: %w", r.Path, err)
	}
	return nil
}

// Args returns the argument list (excluding the binary itself) for a
// print-mode invocation. The prompt is always the final positional argument so
// it is never interpreted by a shell.
func (r *Runner) Args(model string, tools []string, prompt string) []string {
	args := []string{"--print", "--model", model}
	if len(tools) > 0 {
		args = append(args, "--allowedTools", strings.Join(tools, ","))
	}
	return append(args, prompt)
}

// Command builds an *exec.Cmd bound to ctx. When ctx is cancelled — for
// example when the HTTP client disconnects — the child process is killed.
func (r *Runner) Command(ctx context.Context, model string, tools []string, prompt string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, r.Path, r.Args(model, tools, prompt)...)
	// Give a killed process a brief grace period to release its pipes so
	// streaming readers observe EOF promptly.
	cmd.WaitDelay = 5 * time.Second
	return cmd
}
