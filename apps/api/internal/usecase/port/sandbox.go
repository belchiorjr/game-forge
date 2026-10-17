package port

import (
	"context"
	"time"
)

type CommandSpec struct {
	ProjectID string
	Name      string // must be on the adapter's allowlist; never a shell string
	Args      []string
	Timeout   time.Duration
	MaxOutput int
}

type CommandResult struct {
	ExitCode int
	Output   string // trimmed to MaxOutput (head and tail kept)
	TimedOut bool
}

type Sandbox interface {
	Run(ctx context.Context, spec CommandSpec) (CommandResult, error)
}
