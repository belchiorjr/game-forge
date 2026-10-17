package cancelrun

import (
	"context"
	"fmt"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

type Input struct{ ProjectID, RunID string }

type UseCase struct {
	runs port.RunRepository
	sup  port.RunSupervisor
}

func New(runs port.RunRepository, sup port.RunSupervisor) *UseCase {
	return &UseCase{runs: runs, sup: sup}
}

// Execute asks the supervisor to stop the run; the agent loop records the final status.
func (uc *UseCase) Execute(ctx context.Context, in Input) error {
	run, err := uc.runs.Get(ctx, in.ProjectID, in.RunID)
	if err != nil {
		return err
	}
	if run.Status != domain.RunQueued && run.Status != domain.RunRunning {
		return fmt.Errorf("%w: run already %s", domain.ErrConflict, run.Status)
	}
	if !uc.sup.Cancel(run.ID) {
		return fmt.Errorf("%w: run is not owned by this server", domain.ErrConflict)
	}
	return nil
}
