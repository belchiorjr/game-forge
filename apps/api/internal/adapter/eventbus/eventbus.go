// Package eventbus persists events and fans them out to SSE subscribers with replay.
package eventbus

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

const bufferSize = 256

type subscriber struct {
	ch      chan domain.Event
	lastSeq int64 // highest seq already delivered (replay or live), to avoid duplicates
}

type Bus struct {
	repo  port.EventRepository
	clock func() time.Time

	mu   sync.Mutex // also serialises Append so live order matches seq order
	subs map[string]map[*subscriber]struct{}
}

func New(repo port.EventRepository, clock func() time.Time) *Bus {
	return &Bus{repo: repo, clock: clock, subs: map[string]map[*subscriber]struct{}{}}
}

func (b *Bus) Publish(ctx context.Context, projectID, runID string, t domain.EventType, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	e := &domain.Event{ProjectID: projectID, RunID: runID, Type: t, Data: raw, TS: b.clock()}

	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.repo.Append(ctx, e); err != nil {
		return err
	}
	for s := range b.subs[projectID] {
		if e.Seq <= s.lastSeq {
			continue
		}
		select {
		case s.ch <- *e:
			s.lastSeq = e.Seq
		default:
			// Slow subscriber: drop it rather than block the agent. The client reconnects and replays.
			b.removeLocked(projectID, s)
		}
	}
	return nil
}

func (b *Bus) Subscribe(ctx context.Context, projectID string, after int64) (<-chan domain.Event, error) {
	// Register and replay under the lock so no event falls between replay and live.
	b.mu.Lock()
	missed, err := b.repo.After(ctx, projectID, after)
	if err != nil {
		b.mu.Unlock()
		return nil, err
	}
	s := &subscriber{ch: make(chan domain.Event, bufferSize+len(missed)), lastSeq: after}
	for _, e := range missed {
		s.ch <- e
		s.lastSeq = e.Seq
	}
	if b.subs[projectID] == nil {
		b.subs[projectID] = map[*subscriber]struct{}{}
	}
	b.subs[projectID][s] = struct{}{}
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		b.removeLocked(projectID, s)
		b.mu.Unlock()
	}()
	return s.ch, nil
}

func (b *Bus) removeLocked(projectID string, s *subscriber) {
	if _, ok := b.subs[projectID][s]; !ok {
		return
	}
	delete(b.subs[projectID], s)
	close(s.ch)
}
