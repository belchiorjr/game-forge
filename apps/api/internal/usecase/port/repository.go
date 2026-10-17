package port

import (
	"context"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
)

type ProjectRepository interface {
	Save(ctx context.Context, p *domain.Project) error
	Get(ctx context.Context, id string) (*domain.Project, error) // domain.ErrNotFound when missing
	List(ctx context.Context) ([]*domain.Project, error)
}

type RunRepository interface {
	// Save inserts or updates. Returns domain.ErrConflict if another run is active for the project.
	Save(ctx context.Context, r *domain.Run) error
	Get(ctx context.Context, projectID, runID string) (*domain.Run, error)
	List(ctx context.Context, projectID string) ([]*domain.Run, error)
	HasActive(ctx context.Context, projectID string) (bool, error)
	// FailOrphans marks runs left queued/running by a previous process as failed.
	FailOrphans(ctx context.Context) (int, error)
}

type EventRepository interface {
	// Append assigns the next per-project Seq and persists the event.
	Append(ctx context.Context, e *domain.Event) error
	After(ctx context.Context, projectID string, seq int64) ([]domain.Event, error)
}
