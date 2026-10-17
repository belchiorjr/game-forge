package domain

import (
	"fmt"
	"time"
)

type RunStatus string

const (
	RunQueued    RunStatus = "queued"
	RunRunning   RunStatus = "running"
	RunSucceeded RunStatus = "succeeded"
	RunFailed    RunStatus = "failed"
	RunCancelled RunStatus = "cancelled"
)

type Run struct {
	ID        string
	ProjectID string
	Prompt    string
	Status    RunStatus
	StartedAt time.Time
	EndedAt   *time.Time
	Commit    string
}

func NewRun(id, projectID, prompt string, now time.Time) *Run {
	return &Run{ID: id, ProjectID: projectID, Prompt: prompt, Status: RunQueued, StartedAt: now}
}

// Transition keeps the run state machine in one place.
func (r *Run) Transition(to RunStatus, now time.Time) error {
	allowed := map[RunStatus][]RunStatus{
		RunQueued:  {RunRunning, RunCancelled},
		RunRunning: {RunSucceeded, RunFailed, RunCancelled},
	}
	for _, s := range allowed[r.Status] {
		if s == to {
			r.Status = to
			if to != RunRunning {
				r.EndedAt = &now
			}
			return nil
		}
	}
	return fmt.Errorf("%w: cannot go from %s to %s", ErrInvalid, r.Status, to)
}
