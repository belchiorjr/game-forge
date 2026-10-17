package getproject

import (
	"context"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

type Output struct {
	Project   *domain.Project
	LatestRun *domain.Run // nil when the project has no runs
}

type UseCase struct {
	projects port.ProjectRepository
	runs     port.RunRepository
}

func New(projects port.ProjectRepository, runs port.RunRepository) *UseCase {
	return &UseCase{projects: projects, runs: runs}
}

func (uc *UseCase) Execute(ctx context.Context, id string) (Output, error) {
	p, err := uc.projects.Get(ctx, id)
	if err != nil {
		return Output{}, err
	}
	runs, err := uc.runs.List(ctx, id)
	if err != nil {
		return Output{}, err
	}
	out := Output{Project: p}
	if len(runs) > 0 {
		out.LatestRun = runs[0]
	}
	return out, nil
}
