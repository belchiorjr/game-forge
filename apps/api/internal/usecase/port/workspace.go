package port

import "context"

type FileInfo struct {
	Path  string
	IsDir bool
	Size  int64
}

// Workspace is the project's files and git history. Every path is relative to the project root.
type Workspace interface {
	Init(ctx context.Context, projectID, template string) error
	ReadFile(ctx context.Context, projectID, path string) ([]byte, error)
	WriteFile(ctx context.Context, projectID, path string, data []byte) error
	List(ctx context.Context, projectID, dir string) ([]FileInfo, error)
	Commit(ctx context.Context, projectID, message string) (hash string, err error)
	Diff(ctx context.Context, projectID, from, to string) (string, error)
	Revert(ctx context.Context, projectID, hash string) error
}
