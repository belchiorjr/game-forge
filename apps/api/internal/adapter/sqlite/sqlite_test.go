package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
)

func openTest(t *testing.T) (*ProjectRepo, *RunRepo, *EventRepo) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewProjectRepo(db), NewRunRepo(db), NewEventRepo(db)
}

func TestRepos(t *testing.T) {
	ctx := context.Background()
	projects, runs, events := openTest(t)
	now := time.Unix(100, 0)

	p, _ := domain.NewProject("p1", "Runner", "endless runner", "", "", now)
	if err := projects.Save(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := projects.Get(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	r1 := domain.NewRun("r1", "p1", "go", now)
	if err := runs.Save(ctx, r1); err != nil {
		t.Fatal(err)
	}
	// The partial unique index rejects a second active run.
	if err := runs.Save(ctx, domain.NewRun("r2", "p1", "go", now)); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
	_ = r1.Transition(domain.RunRunning, now)
	_ = r1.Transition(domain.RunSucceeded, now)
	r1.Commit = "abc"
	if err := runs.Save(ctx, r1); err != nil {
		t.Fatal(err)
	}
	got, err := runs.Get(ctx, "p1", "r1")
	if err != nil || got.Status != domain.RunSucceeded || got.Commit != "abc" || got.EndedAt == nil {
		t.Fatalf("got %+v, %v", got, err)
	}
	if active, _ := runs.HasActive(ctx, "p1"); active {
		t.Fatal("no run should be active")
	}

	for i := 0; i < 3; i++ {
		e := &domain.Event{ProjectID: "p1", RunID: "r1", Type: domain.EventAgentMessage, Data: []byte(`{}`), TS: now}
		if err := events.Append(ctx, e); err != nil {
			t.Fatal(err)
		}
		if e.Seq != int64(i+1) {
			t.Fatalf("seq = %d, want %d", e.Seq, i+1)
		}
	}
	after, err := events.After(ctx, "p1", 1)
	if err != nil || len(after) != 2 || after[0].Seq != 2 {
		t.Fatalf("after = %+v, %v", after, err)
	}
}
