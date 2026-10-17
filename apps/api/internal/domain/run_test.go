package domain

import (
	"errors"
	"testing"
	"time"
)

func TestRunTransitions(t *testing.T) {
	now := time.Unix(0, 0)
	cases := []struct {
		name    string
		from    RunStatus
		to      RunStatus
		wantErr bool
	}{
		{"queued to running", RunQueued, RunRunning, false},
		{"running to succeeded", RunRunning, RunSucceeded, false},
		{"succeeded is final", RunSucceeded, RunRunning, true},
		{"queued cannot succeed directly", RunQueued, RunSucceeded, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := NewRun("r1", "p1", "make a game", now)
			r.Status = c.from
			err := r.Transition(c.to, now)
			if c.wantErr && !errors.Is(err, ErrInvalid) {
				t.Fatalf("want ErrInvalid, got %v", err)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
