package startrun

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
)

type fakeProjects struct{ exists bool }

func (f fakeProjects) Save(context.Context, *domain.Project) error { return nil }
func (f fakeProjects) Get(context.Context, string) (*domain.Project, error) {
	if !f.exists {
		return nil, domain.ErrNotFound
	}
	return &domain.Project{ID: "p1"}, nil
}
func (f fakeProjects) List(context.Context) ([]*domain.Project, error) { return nil, nil }

type fakeRuns struct {
	active bool
	saved  []*domain.Run
}

func (f *fakeRuns) Save(_ context.Context, r *domain.Run) error {
	f.saved = append(f.saved, r)
	return nil
}
func (f *fakeRuns) Get(context.Context, string, string) (*domain.Run, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeRuns) List(context.Context, string) ([]*domain.Run, error) { return nil, nil }
func (f *fakeRuns) HasActive(context.Context, string) (bool, error)     { return f.active, nil }
func (f *fakeRuns) FailOrphans(context.Context) (int, error)            { return 0, nil }

type fakeSup struct{ started []string }

func (f *fakeSup) Start(r *domain.Run) { f.started = append(f.started, r.ID) }
func (f *fakeSup) Cancel(string) bool  { return false }

func newUC(projectExists, active bool) (*UseCase, *fakeRuns, *fakeSup) {
	runs, sup := &fakeRuns{active: active}, &fakeSup{}
	uc := New(fakeProjects{exists: projectExists}, runs, sup, func() time.Time { return time.Unix(0, 0) }, func() string { return "r1" })
	return uc, runs, sup
}

func TestStartRun(t *testing.T) {
	uc, runs, sup := newUC(true, false)
	out, err := uc.Execute(context.Background(), Input{ProjectID: "p1", Prompt: "make it jump"})
	if err != nil {
		t.Fatal(err)
	}
	if out.RunID != "r1" || len(runs.saved) != 1 || len(sup.started) != 1 {
		t.Fatalf("run not saved/started: %+v %d %d", out, len(runs.saved), len(sup.started))
	}
	if runs.saved[0].Status != domain.RunQueued {
		t.Fatalf("status = %s, want queued", runs.saved[0].Status)
	}
}

func TestStartRunErrors(t *testing.T) {
	cases := []struct {
		name           string
		exists, active bool
		prompt         string
		want           error
	}{
		{"empty prompt", true, false, "  ", domain.ErrInvalid},
		{"missing project", false, false, "go", domain.ErrNotFound},
		{"active run", true, true, "go", domain.ErrConflict},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			uc, _, sup := newUC(c.exists, c.active)
			_, err := uc.Execute(context.Background(), Input{ProjectID: "p1", Prompt: c.prompt})
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if len(sup.started) != 0 {
				t.Fatal("supervisor must not start on error")
			}
		})
	}
}
