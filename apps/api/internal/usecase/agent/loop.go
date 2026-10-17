package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

//go:embed prompt.md
var systemPrompt string

type Config struct {
	MaxSteps    int
	MaxFileSize int
	MaxOutput   int // tool output fed back to the model, in bytes
}

func DefaultConfig() Config {
	return Config{MaxSteps: 60, MaxFileSize: 1 << 20, MaxOutput: 16 << 10}
}

// Loop runs one agent run: LLM call, tool calls, repeat, then commit.
type Loop struct {
	projects port.ProjectRepository
	runs     port.RunRepository
	ws       port.Workspace
	sandbox  port.Sandbox
	llm      port.LLM
	bus      port.EventBus
	clock    func() time.Time
	cfg      Config
	tools    map[string]tool
}

func NewLoop(projects port.ProjectRepository, runs port.RunRepository, ws port.Workspace, sb port.Sandbox, llm port.LLM, bus port.EventBus, clock func() time.Time, cfg Config) *Loop {
	return &Loop{projects: projects, runs: runs, ws: ws, sandbox: sb, llm: llm, bus: bus, clock: clock, cfg: cfg, tools: registry()}
}

// Run drives the run to a final status. ctx is owned by the supervisor (cancel/timeout).
func (l *Loop) Run(ctx context.Context, run *domain.Run) {
	// Bookkeeping must survive cancellation, otherwise a cancelled run stays "running" forever.
	bg := context.WithoutCancel(ctx)
	pub := func(t domain.EventType, data any) { _ = l.bus.Publish(bg, run.ProjectID, run.ID, t, data) }

	if err := run.Transition(domain.RunRunning, l.clock()); err != nil {
		pub(domain.EventError, map[string]any{"code": "internal", "message": err.Error(), "recoverable": false})
		return
	}
	_ = l.runs.Save(bg, run)
	pub(domain.EventRunStarted, map[string]any{"status": run.Status, "prompt": run.Prompt})

	summary, usage, err := l.work(ctx, run)

	status := domain.RunSucceeded
	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		status, summary = domain.RunCancelled, "Cancelled."
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		status, summary = domain.RunFailed, "Timed out."
	case err != nil:
		status, summary = domain.RunFailed, err.Error()
		pub(domain.EventError, map[string]any{"code": "run_failed", "message": err.Error(), "recoverable": true})
	}

	// Every run is a commit, also failed ones (tagged wip) so work is never lost.
	msg := fmt.Sprintf("agent run %s: %s", run.ID, firstLine(summary))
	if status != domain.RunSucceeded {
		msg = "wip: " + msg
	}
	if hash, cerr := l.ws.Commit(bg, run.ProjectID, msg); cerr == nil {
		run.Commit = hash
	} else {
		pub(domain.EventError, map[string]any{"code": "commit_failed", "message": cerr.Error(), "recoverable": true})
	}
	_ = run.Transition(status, l.clock())
	_ = l.runs.Save(bg, run)
	pub(domain.EventRunFinished, map[string]any{"status": run.Status, "summary": summary, "commit": run.Commit,
		"usage": map[string]int{"input_tokens": usage.InputTokens, "output_tokens": usage.OutputTokens}})
}

func (l *Loop) work(ctx context.Context, run *domain.Run) (string, port.Usage, error) {
	var total port.Usage
	project, err := l.projects.Get(ctx, run.ProjectID)
	if err != nil {
		return "", total, err
	}
	env := &toolEnv{ctx: ctx, projectID: run.ProjectID, runID: run.ID, ws: l.ws, sandbox: l.sandbox, bus: l.bus, maxFile: l.cfg.MaxFileSize, passed: map[string]bool{}}
	msgs := []port.Message{{Role: "user", Text: l.orientation(ctx, project, run.Prompt)}}
	defs := l.toolDefs()

	for step := 0; step < l.cfg.MaxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return "", total, err
		}
		resp, err := l.llm.Complete(ctx, systemPrompt, msgs, defs)
		if err != nil {
			return "", total, fmt.Errorf("llm: %w", err)
		}
		total.InputTokens += resp.Usage.InputTokens
		total.OutputTokens += resp.Usage.OutputTokens
		env.publish(domain.EventUsage, map[string]int{"input_tokens": total.InputTokens, "output_tokens": total.OutputTokens})

		if resp.Text != "" {
			env.publish(domain.EventAgentMessage, map[string]string{"text": resp.Text})
		}
		msgs = append(msgs, port.Message{Role: "assistant", Text: resp.Text, ToolCalls: resp.ToolCalls, Opaque: resp.Opaque})
		if len(resp.ToolCalls) == 0 {
			// Plain text without finish: treat as the final answer (often a question for the person).
			return resp.Text, total, nil
		}
		for _, call := range resp.ToolCalls {
			out, ok := l.execTool(env, call)
			msgs = append(msgs, port.Message{Role: "tool", ToolID: call.ID, Text: out, IsError: !ok})
		}
		if env.finished {
			return env.summary, total, nil
		}
	}
	return "", total, fmt.Errorf("%w: step limit of %d reached", domain.ErrLimitExceeded, l.cfg.MaxSteps)
}

func (l *Loop) execTool(env *toolEnv, call port.ToolCall) (string, bool) {
	start := l.clock()
	var target struct{ Path string }
	_ = json.Unmarshal(call.Input, &target) // file tools carry a path the UI shows even when args are cut
	env.publish(domain.EventToolStarted, map[string]string{"tool_call_id": call.ID, "name": call.Name, "path": target.Path, "args": truncate(string(call.Input), 300)})
	t, ok := l.tools[call.Name]
	var out string
	var err error
	if !ok {
		err = fmt.Errorf("unknown tool %q", call.Name)
	} else {
		out, err = t.handler(env, call.Input)
	}
	ok = err == nil
	if err != nil {
		out = "error: " + err.Error()
	}
	out = truncate(out, l.cfg.MaxOutput)
	env.publish(domain.EventToolFinished, map[string]any{
		"tool_call_id": call.ID, "name": call.Name, "ok": ok,
		"output": truncate(out, 500), "duration_ms": l.clock().Sub(start).Milliseconds(),
	})
	return out, ok
}

func (l *Loop) toolDefs() []port.ToolDef {
	defs := make([]port.ToolDef, 0, len(l.tools))
	for _, t := range l.tools {
		defs = append(defs, t.def)
	}
	// Stable order keeps the prompt prefix cacheable.
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	return defs
}

// orientation gives the model the project memory and file tree up front so a follow-up
// run starts oriented instead of re-deriving everything.
func (l *Loop) orientation(ctx context.Context, p *domain.Project, prompt string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Project: %s\nIdea: %s\nGenre: %s\nOrientation: %s\n\n", p.Name, p.Idea, p.Genre, p.Orientation)
	for _, f := range []string{"docs/GDD.md", "docs/AGENT_NOTES.md"} {
		if data, err := l.ws.ReadFile(ctx, p.ID, f); err == nil {
			fmt.Fprintf(&b, "--- %s ---\n%s\n\n", f, truncate(string(data), 8<<10))
		}
	}
	if files, err := l.ws.List(ctx, p.ID, ""); err == nil {
		b.WriteString("--- file tree ---\n")
		for _, f := range files {
			if !f.IsDir {
				b.WriteString(f.Path + "\n")
			}
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "--- request ---\n%s\n", prompt)
	return b.String()
}

// truncate keeps head and tail so error messages at the end of long outputs survive.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	half := max / 2
	return s[:half] + "\n…[truncated]…\n" + s[len(s)-half:]
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	if len(s) > 72 {
		s = s[:71] + "…"
	}
	return s
}
