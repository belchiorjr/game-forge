package startrun

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

type Input struct{ ProjectID, Prompt string }
type Output struct{ RunID string }

type UseCase struct {
	projects port.ProjectRepository
	runs     port.RunRepository
	sup      port.RunSupervisor
	clock    func() time.Time
	newID    func() string
}

func New(projects port.ProjectRepository, runs port.RunRepository, sup port.RunSupervisor, clock func() time.Time, newID func() string) *UseCase {
	return &UseCase{projects: projects, runs: runs, sup: sup, clock: clock, newID: newID}
}

func (uc *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	if strings.TrimSpace(in.Prompt) == "" {
		return Output{}, fmt.Errorf("%w: empty prompt", domain.ErrInvalid)
	}
	if _, err := uc.projects.Get(ctx, in.ProjectID); err != nil {
		return Output{}, err
	}
	active, err := uc.runs.HasActive(ctx, in.ProjectID)
	if err != nil {
		return Output{}, err
	}
	if active {
		return Output{}, fmt.Errorf("%w: run_active: a run is already active for this project", domain.ErrConflict)
	}
	run := domain.NewRun(uc.newID(), in.ProjectID, in.Prompt, uc.clock())
	// The repository enforces one active run per project too, so a race still fails with ErrConflict.
	if err := uc.runs.Save(ctx, run); err != nil {
		return Output{}, err
	}
	uc.sup.Start(run)
	return Output{RunID: run.ID}, nil
}
