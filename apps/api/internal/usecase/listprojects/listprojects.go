package listprojects

import (
	"context"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

type UseCase struct{ projects port.ProjectRepository }

func New(projects port.ProjectRepository) *UseCase { return &UseCase{projects: projects} }

func (uc *UseCase) Execute(ctx context.Context) ([]*domain.Project, error) {
	return uc.projects.List(ctx)
}
