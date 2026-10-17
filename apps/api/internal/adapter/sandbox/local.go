// Package sandbox runs allowlisted commands inside a project workspace. No shell is ever used:
// the agent's code is untrusted, so commands get a scrubbed environment, a timeout that kills
// the whole process group, and capped output.
package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

var (
	idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	// Only these commands, each with only these first arguments.
	allowed = map[string][]string{
		"go":    {"build", "vet", "test", "env", "mod"},
		"gofmt": {"-l"},
	}
	allowedEnv = map[string]bool{"GOOS": true, "GOARCH": true, "CGO_ENABLED": true}
)

type Local struct {
	root       string // workspaces directory
	goCache    string
	goModCache string
	path       string // PATH used for commands
}

func NewLocal(workspaces, goCache, goModCache string) (*Local, error) {
	root, err := filepath.Abs(workspaces)
	if err != nil {
		return nil, err
	}
	for _, d := range []string{goCache, goModCache} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	return &Local{root: root, goCache: goCache, goModCache: goModCache, path: os.Getenv("PATH")}, nil
}

func (l *Local) Run(ctx context.Context, spec port.CommandSpec) (port.CommandResult, error) {
	if !idPattern.MatchString(spec.ProjectID) {
		return port.CommandResult{}, fmt.Errorf("%w: bad project id", domain.ErrInvalid)
	}
	subs, ok := allowed[spec.Name]
	if !ok || len(spec.Args) == 0 || !contains(subs, spec.Args[0]) {
		return port.CommandResult{}, fmt.Errorf("%w: command %q %v is not allowed", domain.ErrInvalid, spec.Name, spec.Args)
	}
	for _, a := range spec.Args {
		if strings.HasPrefix(a, "-toolexec") || strings.HasPrefix(a, "-exec") || strings.Contains(a, "../") || a == ".." || filepath.IsAbs(a) {
			return port.CommandResult{}, fmt.Errorf("%w: argument %q is not allowed", domain.ErrInvalid, a)
		}
	}
	bin, err := exec.LookPath(spec.Name)
	if err != nil {
		return port.CommandResult{}, err
	}
	home, err := os.MkdirTemp("", "forge-home-")
	if err != nil {
		return port.CommandResult{}, err
	}
	defer os.RemoveAll(home)

	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.Command(bin, spec.Args...)
	cmd.Dir = filepath.Join(l.root, spec.ProjectID)
	// Scrubbed environment: no API keys or tokens from the server process reach agent code.
	cmd.Env = []string{
		"PATH=" + l.path, "HOME=" + home, "TMPDIR=" + home,
		"GOCACHE=" + l.goCache, "GOMODCACHE=" + l.goModCache,
		"GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "GOTELEMETRY=off", "CGO_ENABLED=0",
	}
	for k, v := range spec.Env {
		if !allowedEnv[k] {
			return port.CommandResult{}, fmt.Errorf("%w: env %s is not allowed", domain.ErrInvalid, k)
		}
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out

	if err := cmd.Start(); err != nil {
		return port.CommandResult{}, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	res := port.CommandResult{}
	select {
	case err = <-done:
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) // kill the whole group, not just go
		err = <-done
		res.TimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
	}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		return res, err
	}
	res.Output = capOutput(out.String(), spec.MaxOutput)
	return res, nil
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// capOutput keeps head and tail: compiler errors are usually at the start, panics at the end.
func capOutput(s string, max int) string {
	if max <= 0 {
		max = 32 << 10
	}
	if len(s) <= max {
		return s
	}
	half := max / 2
	return s[:half] + fmt.Sprintf("\n…[%d bytes cut]…\n", len(s)-max) + s[len(s)-half:]
}
