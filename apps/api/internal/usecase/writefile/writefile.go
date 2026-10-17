package writefile

import (
	"context"
	"fmt"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

const maxFileSize = 1 << 20

type Input struct {
	ProjectID, Path string
	Content         []byte
}

type Output struct{ Commit string }

type UseCase struct {
	projects port.ProjectRepository
	runs     port.RunRepository
	ws       port.Workspace
	bus      port.EventBus
}

func New(projects port.ProjectRepository, runs port.RunRepository, ws port.Workspace, bus port.EventBus) *UseCase {
	return &UseCase{projects: projects, runs: runs, ws: ws, bus: bus}
}

// Execute saves a manual edit as its own commit. It is refused while the agent is running,
// because two writers on one workspace corrupt it.
func (uc *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	if _, err := uc.projects.Get(ctx, in.ProjectID); err != nil {
		return Output{}, err
	}
	if len(in.Content) > maxFileSize {
		return Output{}, fmt.Errorf("%w: file larger than 1 MB", domain.ErrInvalid)
	}
	active, err := uc.runs.HasActive(ctx, in.ProjectID)
	if err != nil {
		return Output{}, err
	}
	if active {
		return Output{}, fmt.Errorf("%w: edit_locked: the agent is working on this project", domain.ErrConflict)
	}
	if err := uc.ws.WriteFile(ctx, in.ProjectID, in.Path, in.Content); err != nil {
		return Output{}, err
	}
	hash, err := uc.ws.Commit(ctx, in.ProjectID, "manual edit: "+in.Path)
	if err != nil {
		return Output{}, err
	}
	_ = uc.bus.Publish(ctx, in.ProjectID, "", domain.EventFileChanged, map[string]string{"path": in.Path, "kind": "modified"})
	return Output{Commit: hash}, nil
}
