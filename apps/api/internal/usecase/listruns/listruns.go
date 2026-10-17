package listruns

import (
	"context"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

type UseCase struct {
	projects port.ProjectRepository
	runs     port.RunRepository
}

func New(projects port.ProjectRepository, runs port.RunRepository) *UseCase {
	return &UseCase{projects: projects, runs: runs}
}

// Execute returns the project's runs, newest first.
func (uc *UseCase) Execute(ctx context.Context, projectID string) ([]*domain.Run, error) {
	if _, err := uc.projects.Get(ctx, projectID); err != nil {
		return nil, err
	}
	return uc.runs.List(ctx, projectID)
}
