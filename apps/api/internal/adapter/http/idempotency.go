package http

import (
	"sync"
	"time"
)

// idempotency remembers Idempotency-Key -> run ID so a retried POST /runs does not start a second run.
type idempotency struct {
	ttl time.Duration
	mu  sync.Mutex
	m   map[string]idemEntry
}

type idemEntry struct {
	runID string
	at    time.Time
}

func newIdempotency(ttl time.Duration) *idempotency {
	return &idempotency{ttl: ttl, m: map[string]idemEntry{}}
}

func (i *idempotency) get(projectID, key string) (string, bool) {
	if key == "" {
		return "", false
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	e, ok := i.m[projectID+"|"+key]
	if !ok || time.Since(e.at) > i.ttl {
		return "", false
	}
	return e.runID, true
}

func (i *idempotency) put(projectID, key, runID string) {
	if key == "" {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	now := time.Now()
	for k, e := range i.m {
		if now.Sub(e.at) > i.ttl {
			delete(i.m, k)
		}
	}
	i.m[projectID+"|"+key] = idemEntry{runID: runID, at: now}
}
