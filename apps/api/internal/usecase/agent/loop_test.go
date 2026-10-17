package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

// ---- fakes ----

type memWS struct {
	mu      sync.Mutex
	files   map[string]string
	commits []string
}

func (w *memWS) Init(context.Context, string, string) error { return nil }
func (w *memWS) ReadFile(_ context.Context, _, p string) ([]byte, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	f, ok := w.files[p]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return []byte(f), nil
}
func (w *memWS) WriteFile(_ context.Context, _, p string, d []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.files[p] = string(d)
	return nil
}
func (w *memWS) List(context.Context, string, string) ([]port.FileInfo, error) { return nil, nil }
func (w *memWS) Commit(_ context.Context, _, msg string) (string, error) {
	w.commits = append(w.commits, msg)
	return "deadbeef", nil
}
func (w *memWS) Diff(context.Context, string, string, string) (string, error) { return "", nil }
func (w *memWS) Revert(context.Context, string, string) error                 { return nil }

type memRuns struct{ last *domain.Run }

func (r *memRuns) Save(_ context.Context, run *domain.Run) error            { c := *run; r.last = &c; return nil }
func (r *memRuns) Get(context.Context, string, string) (*domain.Run, error) { return r.last, nil }
func (r *memRuns) List(context.Context, string) ([]*domain.Run, error)      { return nil, nil }
func (r *memRuns) HasActive(context.Context, string) (bool, error)          { return false, nil }
func (r *memRuns) FailOrphans(context.Context) (int, error)                 { return 0, nil }

type oneProject struct{}

func (oneProject) Save(context.Context, *domain.Project) error { return nil }
func (oneProject) Get(context.Context, string) (*domain.Project, error) {
	return &domain.Project{ID: "p1", Name: "Runner", Idea: "runner"}, nil
}
func (oneProject) List(context.Context) ([]*domain.Project, error) { return nil, nil }

type recBus struct {
	mu     sync.Mutex
	events []domain.EventType
	data   []string
}

func (b *recBus) Publish(_ context.Context, _, _ string, t domain.EventType, d any) error {
	raw, _ := json.Marshal(d)
	b.mu.Lock()
	b.events, b.data = append(b.events, t), append(b.data, string(raw))
	b.mu.Unlock()
	return nil
}
func (b *recBus) Subscribe(context.Context, string, int64) (<-chan domain.Event, error) {
	return nil, nil
}

// scripted returns the given responses in order, then repeats the last one.
type scripted struct {
	steps   []port.LLMResponse
	calls   int
	block   chan struct{} // when set, Complete waits on it (or ctx) before answering
	started chan struct{} // closed on the first call when block is set
	seen    [][]port.Message
}

func (s *scripted) Complete(ctx context.Context, _ string, msgs []port.Message, _ []port.ToolDef) (port.LLMResponse, error) {
	s.seen = append(s.seen, append([]port.Message(nil), msgs...))
	if s.block != nil {
		if len(s.seen) == 1 {
			close(s.started)
		}
		select {
		case <-s.block:
		case <-ctx.Done():
			return port.LLMResponse{}, ctx.Err()
		}
	}
	i := min(s.calls, len(s.steps)-1)
	s.calls++
	return s.steps[i], nil
}

func call(id, name, input string) port.LLMResponse {
	return port.LLMResponse{ToolCalls: []port.ToolCall{{ID: id, Name: name, Input: []byte(input)}}}
}

func newTestLoop(llm port.LLM, maxSteps int) (*Loop, *memWS, *memRuns, *recBus) {
	ws, runs, bus := &memWS{files: map[string]string{"docs/GDD.md": "# GDD"}}, &memRuns{}, &recBus{}
	cfg := DefaultConfig()
	cfg.MaxSteps = maxSteps
	return NewLoop(oneProject{}, runs, ws, llm, bus, func() time.Time { return time.Unix(0, 0) }, cfg), ws, runs, bus
}

// ---- tests ----

func TestLoopSuccess(t *testing.T) {
	llm := &scripted{steps: []port.LLMResponse{
		call("1", "write_file", `{"path":"internal/logic/a.go","content":"package logic"}`),
		call("2", "edit_file", `{"path":"internal/logic/a.go","old":"logic","new":"logic // ok"}`),
		call("3", "finish", `{"summary":"done"}`),
	}}
	loop, ws, runs, bus := newTestLoop(llm, 10)
	loop.Run(context.Background(), domain.NewRun("r1", "p1", "make it", time.Unix(0, 0)))

	if runs.last.Status != domain.RunSucceeded || runs.last.Commit != "deadbeef" {
		t.Fatalf("run = %+v", runs.last)
	}
	if got := ws.files["internal/logic/a.go"]; got != "package logic // ok" {
		t.Fatalf("file = %q", got)
	}
	if len(ws.commits) != 1 || strings.HasPrefix(ws.commits[0], "wip") {
		t.Fatalf("commits = %v", ws.commits)
	}
	if bus.events[0] != domain.EventRunStarted || bus.events[len(bus.events)-1] != domain.EventRunFinished {
		t.Fatalf("events = %v", bus.events)
	}
}

func TestLoopToolErrorIsFedBack(t *testing.T) {
	llm := &scripted{steps: []port.LLMResponse{
		call("1", "edit_file", `{"path":"docs/GDD.md","old":"missing","new":"x"}`),
		call("2", "finish", `{"summary":"done"}`),
	}}
	loop, _, runs, _ := newTestLoop(llm, 10)
	loop.Run(context.Background(), domain.NewRun("r1", "p1", "go", time.Unix(0, 0)))

	last := llm.seen[1][len(llm.seen[1])-1]
	if last.Role != "tool" || !last.IsError || !strings.Contains(last.Text, "exactly once") {
		t.Fatalf("tool error not fed back: %+v", last)
	}
	if runs.last.Status != domain.RunSucceeded {
		t.Fatalf("status = %s", runs.last.Status)
	}
}

func TestLoopStepLimit(t *testing.T) {
	llm := &scripted{steps: []port.LLMResponse{call("1", "list_files", `{}`)}}
	loop, ws, runs, _ := newTestLoop(llm, 3)
	loop.Run(context.Background(), domain.NewRun("r1", "p1", "go", time.Unix(0, 0)))

	if runs.last.Status != domain.RunFailed || llm.calls != 3 {
		t.Fatalf("status = %s, calls = %d", runs.last.Status, llm.calls)
	}
	// Work is still committed, tagged wip.
	if len(ws.commits) != 1 || !strings.HasPrefix(ws.commits[0], "wip:") {
		t.Fatalf("commits = %v", ws.commits)
	}
}

func TestLoopCancel(t *testing.T) {
	llm := &scripted{steps: []port.LLMResponse{call("1", "finish", `{"summary":"x"}`)}, block: make(chan struct{}), started: make(chan struct{})}
	loop, _, runs, _ := newTestLoop(llm, 10)
	sup := NewSupervisor(loop, time.Minute)
	run := domain.NewRun("r1", "p1", "go", time.Unix(0, 0))
	sup.Start(run)
	<-llm.started
	if !sup.Cancel("r1") {
		t.Fatal("cancel returned false")
	}
	sup.Shutdown()
	if runs.last.Status != domain.RunCancelled {
		t.Fatalf("status = %s", runs.last.Status)
	}
}
