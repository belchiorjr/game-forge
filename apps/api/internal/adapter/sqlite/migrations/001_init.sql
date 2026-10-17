CREATE TABLE IF NOT EXISTS projects (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  idea        TEXT NOT NULL,
  genre       TEXT NOT NULL DEFAULT '',
  orientation TEXT NOT NULL,
  created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS runs (
  id         TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id),
  prompt     TEXT NOT NULL,
  status     TEXT NOT NULL,
  started_at TEXT NOT NULL,
  ended_at   TEXT,
  commit_sha TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS runs_project ON runs(project_id, started_at);
-- One active run per project, enforced by the database so a race cannot create two.
CREATE UNIQUE INDEX IF NOT EXISTS runs_one_active ON runs(project_id) WHERE status IN ('queued','running');

CREATE TABLE IF NOT EXISTS events (
  project_id TEXT NOT NULL,
  seq        INTEGER NOT NULL,
  run_id     TEXT NOT NULL DEFAULT '',
  type       TEXT NOT NULL,
  data       TEXT NOT NULL,
  ts         TEXT NOT NULL,
  PRIMARY KEY (project_id, seq)
);
