package streamevents

import (
	"context"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

type UseCase struct {
	projects port.ProjectRepository
	bus      port.EventBus
}

func New(projects port.ProjectRepository, bus port.EventBus) *UseCase {
	return &UseCase{projects: projects, bus: bus}
}

// Execute replays events after lastSeq, then streams live ones until ctx ends.
func (uc *UseCase) Execute(ctx context.Context, projectID string, lastSeq int64) (<-chan domain.Event, error) {
	if _, err := uc.projects.Get(ctx, projectID); err != nil {
		return nil, err
	}
	return uc.bus.Subscribe(ctx, projectID, lastSeq)
}
