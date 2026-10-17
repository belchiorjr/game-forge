// Package buildpreview builds the WASM preview on request (the "Build preview" button),
// outside an agent run.
package buildpreview

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

type UseCase struct {
	projects port.ProjectRepository
	runs     port.RunRepository
	sandbox  port.Sandbox
	bus      port.EventBus

	mu       sync.Mutex
	building map[string]bool
}

func New(projects port.ProjectRepository, runs port.RunRepository, sb port.Sandbox, bus port.EventBus) *UseCase {
	return &UseCase{projects: projects, runs: runs, sandbox: sb, bus: bus, building: map[string]bool{}}
}

// Execute starts the build in the background; progress arrives as build.* events.
func (uc *UseCase) Execute(ctx context.Context, projectID string) error {
	if _, err := uc.projects.Get(ctx, projectID); err != nil {
		return err
	}
	active, err := uc.runs.HasActive(ctx, projectID)
	if err != nil {
		return err
	}
	if active {
		return fmt.Errorf("%w: run_active: the agent is building this project", domain.ErrConflict)
	}
	uc.mu.Lock()
	if uc.building[projectID] {
		uc.mu.Unlock()
		return fmt.Errorf("%w: build_active: a build is already running", domain.ErrConflict)
	}
	uc.building[projectID] = true
	uc.mu.Unlock()

	go func() {
		defer func() {
			uc.mu.Lock()
			delete(uc.building, projectID)
			uc.mu.Unlock()
		}()
		bg := context.Background()
		_ = uc.bus.Publish(bg, projectID, "", domain.EventBuildStarted, map[string]string{"target": "wasm"})
		start := time.Now()
		res, err := uc.sandbox.Run(bg, port.CommandSpec{
			ProjectID: projectID, Name: "go", Args: []string{"build", "-o", "web/game.wasm", "./cmd/wasm"},
			Env: map[string]string{"GOOS": "js", "GOARCH": "wasm"}, Timeout: 3 * time.Minute, MaxOutput: 8 << 10,
		})
		out := res.Output
		if err != nil {
			out = err.Error()
		}
		_ = uc.bus.Publish(bg, projectID, "", domain.EventBuildFinished, map[string]any{
			"target": "wasm", "ok": err == nil && res.ExitCode == 0 && !res.TimedOut,
			"duration_ms": time.Since(start).Milliseconds(), "output": out,
		})
	}()
	return nil
}
