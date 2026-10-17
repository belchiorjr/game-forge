package http

import (
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

type projectDTO struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Idea        string    `json:"idea"`
	Genre       string    `json:"genre"`
	Orientation string    `json:"orientation"`
	CreatedAt   time.Time `json:"created_at"`
	LatestRun   *runDTO   `json:"latest_run,omitempty"`
}

type runDTO struct {
	ID        string     `json:"id"`
	ProjectID string     `json:"project_id"`
	Prompt    string     `json:"prompt"`
	Status    string     `json:"status"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
	Commit    string     `json:"commit"`
}

type fileDTO struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

func toProject(p *domain.Project) projectDTO {
	return projectDTO{ID: p.ID, Name: p.Name, Idea: p.Idea, Genre: p.Genre, Orientation: string(p.Orientation), CreatedAt: p.CreatedAt}
}

func toRun(r *domain.Run) *runDTO {
	if r == nil {
		return nil
	}
	return &runDTO{ID: r.ID, ProjectID: r.ProjectID, Prompt: r.Prompt, Status: string(r.Status), StartedAt: r.StartedAt, EndedAt: r.EndedAt, Commit: r.Commit}
}

func toFiles(fs []port.FileInfo) []fileDTO {
	out := make([]fileDTO, len(fs))
	for i, f := range fs {
		out[i] = fileDTO{Path: f.Path, IsDir: f.IsDir, Size: f.Size}
	}
	return out
}
