// Package sqlite implements the repositories on SQLite through the pure-Go modernc driver.
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// A single writer connection avoids SQLITE_BUSY under concurrent runs.
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, n := range names {
		q, err := migrations.ReadFile("migrations/" + n)
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(q)); err != nil {
			return fmt.Errorf("migration %s: %w", n, err)
		}
	}
	return nil
}

const timeFmt = time.RFC3339Nano

func fmtTime(t time.Time) string { return t.UTC().Format(timeFmt) }

func parseTime(s string) time.Time {
	t, _ := time.Parse(timeFmt, s)
	return t
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// ---------- projects ----------

type ProjectRepo struct{ db *sql.DB }

func NewProjectRepo(db *sql.DB) *ProjectRepo { return &ProjectRepo{db: db} }

func (r *ProjectRepo) Save(ctx context.Context, p *domain.Project) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO projects (id, name, idea, genre, orientation, created_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name = excluded.name, idea = excluded.idea, genre = excluded.genre, orientation = excluded.orientation`,
		p.ID, p.Name, p.Idea, p.Genre, string(p.Orientation), fmtTime(p.CreatedAt))
	return err
}

func (r *ProjectRepo) Get(ctx context.Context, id string) (*domain.Project, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, idea, genre, orientation, created_at FROM projects WHERE id = ?`, id)
	p, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: project %s", domain.ErrNotFound, id)
	}
	return p, err
}

func (r *ProjectRepo) List(ctx context.Context) ([]*domain.Project, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, idea, genre, orientation, created_at FROM projects ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*domain.Project{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scanProject(s scanner) (*domain.Project, error) {
	var p domain.Project
	var orientation, created string
	if err := s.Scan(&p.ID, &p.Name, &p.Idea, &p.Genre, &orientation, &created); err != nil {
		return nil, err
	}
	p.Orientation, p.CreatedAt = domain.Orientation(orientation), parseTime(created)
	return &p, nil
}

// ---------- runs ----------

type RunRepo struct{ db *sql.DB }

func NewRunRepo(db *sql.DB) *RunRepo { return &RunRepo{db: db} }

func (r *RunRepo) Save(ctx context.Context, run *domain.Run) error {
	var ended any
	if run.EndedAt != nil {
		ended = fmtTime(*run.EndedAt)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO runs (id, project_id, prompt, status, started_at, ended_at, commit_sha) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET status = excluded.status, ended_at = excluded.ended_at, commit_sha = excluded.commit_sha`,
		run.ID, run.ProjectID, run.Prompt, string(run.Status), fmtTime(run.StartedAt), ended, run.Commit)
	if isUniqueViolation(err) {
		return fmt.Errorf("%w: run_active: a run is already active for this project", domain.ErrConflict)
	}
	return err
}

const runCols = `id, project_id, prompt, status, started_at, ended_at, commit_sha`

func (r *RunRepo) Get(ctx context.Context, projectID, runID string) (*domain.Run, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+runCols+` FROM runs WHERE project_id = ? AND id = ?`, projectID, runID)
	run, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: run %s", domain.ErrNotFound, runID)
	}
	return run, err
}

func (r *RunRepo) List(ctx context.Context, projectID string) ([]*domain.Run, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+runCols+` FROM runs WHERE project_id = ? ORDER BY started_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*domain.Run{}
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func (r *RunRepo) HasActive(ctx context.Context, projectID string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM runs WHERE project_id = ? AND status IN ('queued','running')`, projectID).Scan(&n)
	return n > 0, err
}

func (r *RunRepo) FailOrphans(ctx context.Context) (int, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE runs SET status = 'failed', ended_at = ? WHERE status IN ('queued','running')`, fmtTime(time.Now()))
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func scanRun(s scanner) (*domain.Run, error) {
	var run domain.Run
	var status, started string
	var ended sql.NullString
	if err := s.Scan(&run.ID, &run.ProjectID, &run.Prompt, &status, &started, &ended, &run.Commit); err != nil {
		return nil, err
	}
	run.Status, run.StartedAt = domain.RunStatus(status), parseTime(started)
	if ended.Valid {
		t := parseTime(ended.String)
		run.EndedAt = &t
	}
	return &run, nil
}

// ---------- events ----------

type EventRepo struct{ db *sql.DB }

func NewEventRepo(db *sql.DB) *EventRepo { return &EventRepo{db: db} }

func (r *EventRepo) Append(ctx context.Context, e *domain.Event) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) + 1 FROM events WHERE project_id = ?`, e.ProjectID).Scan(&e.Seq); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO events (project_id, seq, run_id, type, data, ts) VALUES (?, ?, ?, ?, ?, ?)`,
		e.ProjectID, e.Seq, e.RunID, string(e.Type), string(e.Data), fmtTime(e.TS)); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *EventRepo) After(ctx context.Context, projectID string, seq int64) ([]domain.Event, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT project_id, seq, run_id, type, data, ts FROM events WHERE project_id = ? AND seq > ? ORDER BY seq`, projectID, seq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Event
	for rows.Next() {
		var e domain.Event
		var typ, data, ts string
		if err := rows.Scan(&e.ProjectID, &e.Seq, &e.RunID, &typ, &data, &ts); err != nil {
			return nil, err
		}
		e.Type, e.Data, e.TS = domain.EventType(typ), []byte(data), parseTime(ts)
		out = append(out, e)
	}
	return out, rows.Err()
}
