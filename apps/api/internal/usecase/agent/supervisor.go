package agent

import (
	"context"
	"sync"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
)

type runner interface {
	Run(ctx context.Context, run *domain.Run)
}

// Supervisor owns run contexts: runs outlive the HTTP request that started them.
type Supervisor struct {
	runner  runner
	timeout time.Duration

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
	wg      sync.WaitGroup
}

func NewSupervisor(r runner, timeout time.Duration) *Supervisor {
	return &Supervisor{runner: r, timeout: timeout, cancels: map[string]context.CancelFunc{}}
}

func (s *Supervisor) Start(run *domain.Run) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	s.mu.Lock()
	s.cancels[run.ID] = cancel
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			s.mu.Lock()
			delete(s.cancels, run.ID)
			s.mu.Unlock()
			cancel()
		}()
		s.runner.Run(ctx, run)
	}()
}

func (s *Supervisor) Cancel(runID string) bool {
	s.mu.Lock()
	cancel, ok := s.cancels[runID]
	s.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

// Shutdown cancels every run and waits for them to record their final status.
func (s *Supervisor) Shutdown() {
	s.mu.Lock()
	for _, c := range s.cancels {
		c()
	}
	s.mu.Unlock()
	s.wg.Wait()
}
