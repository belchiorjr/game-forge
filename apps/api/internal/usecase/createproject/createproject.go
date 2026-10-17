package createproject

import (
	"context"
	"fmt"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

type Input struct {
	Name, Idea, Genre, Orientation string
}

type Output struct{ Project *domain.Project }

type UseCase struct {
	projects port.ProjectRepository
	ws       port.Workspace
	template string
	clock    func() time.Time
	newID    func() string
}

func New(projects port.ProjectRepository, ws port.Workspace, template string, clock func() time.Time, newID func() string) *UseCase {
	return &UseCase{projects: projects, ws: ws, template: template, clock: clock, newID: newID}
}

// Execute validates the idea, copies the game template into a fresh git workspace and stores the project.
func (uc *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	p, err := domain.NewProject(uc.newID(), in.Name, in.Idea, in.Genre, domain.Orientation(in.Orientation), uc.clock())
	if err != nil {
		return Output{}, err
	}
	if err := uc.ws.Init(ctx, p.ID, uc.template); err != nil {
		return Output{}, fmt.Errorf("init workspace: %w", err)
	}
	if err := uc.projects.Save(ctx, p); err != nil {
		return Output{}, err
	}
	return Output{Project: p}, nil
}
