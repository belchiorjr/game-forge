package getrun

import (
	"context"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

type UseCase struct{ runs port.RunRepository }

func New(runs port.RunRepository) *UseCase { return &UseCase{runs: runs} }

func (uc *UseCase) Execute(ctx context.Context, projectID, runID string) (*domain.Run, error) {
	return uc.runs.Get(ctx, projectID, runID)
}
