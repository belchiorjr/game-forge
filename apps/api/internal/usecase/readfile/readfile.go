package readfile

import (
	"context"

	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

type UseCase struct {
	projects port.ProjectRepository
	ws       port.Workspace
}

func New(projects port.ProjectRepository, ws port.Workspace) *UseCase {
	return &UseCase{projects: projects, ws: ws}
}

func (uc *UseCase) Execute(ctx context.Context, projectID, path string) ([]byte, error) {
	if _, err := uc.projects.Get(ctx, projectID); err != nil {
		return nil, err
	}
	return uc.ws.ReadFile(ctx, projectID, path)
}
