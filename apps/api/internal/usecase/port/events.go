package port

import (
	"context"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
)

// EventBus persists events and fans them out to live subscribers.
type EventBus interface {
	Publish(ctx context.Context, projectID, runID string, t domain.EventType, data any) error
	// Subscribe replays events after `after` and then streams live ones until ctx ends.
	// The channel is closed when the subscriber is dropped (too slow) or ctx ends.
	Subscribe(ctx context.Context, projectID string, after int64) (<-chan domain.Event, error)
}

// RunSupervisor owns detached run contexts.
type RunSupervisor interface {
	Start(run *domain.Run)
	Cancel(runID string) bool
}
