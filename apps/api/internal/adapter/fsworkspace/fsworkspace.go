// Package fsworkspace stores each project as a path-jailed directory that is its own git repo.
package fsworkspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

var (
	idPattern   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	hashPattern = regexp.MustCompile(`^[0-9a-f]{4,64}$`)
	skipDirs    = map[string]bool{".git": true, "out": true, "node_modules": true}
)

type Workspace struct {
	root      string // workspaces directory
	templates string // directory that holds templates by name
}

func New(root, templates string) (*Workspace, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	// Resolve symlinks once so the jail prefix check compares real paths (macOS /var -> /private/var).
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	tpl, err := filepath.Abs(templates)
	if err != nil {
		return nil, err
	}
	return &Workspace{root: abs, templates: tpl}, nil
}

func (w *Workspace) projectDir(projectID string) (string, error) {
	if !idPattern.MatchString(projectID) {
		return "", fmt.Errorf("%w: bad project id", domain.ErrInvalid)
	}
	return filepath.Join(w.root, projectID), nil
}

// resolve maps a relative path into the project and refuses anything that escapes it,
// including through symlinks.
func (w *Workspace) resolve(projectID, rel string) (string, error) {
	dir, err := w.projectDir(projectID)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("%w: absolute paths are not allowed", domain.ErrInvalid)
	}
	if clean := filepath.Clean(rel); clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%w: path escapes the project", domain.ErrInvalid)
	}
	p := filepath.Join(dir, rel)
	if p != dir && !strings.HasPrefix(p, dir+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: path escapes the project", domain.ErrInvalid)
	}
	if rel := strings.TrimPrefix(p, dir); strings.HasPrefix(rel, "/.git/") || rel == "/.git" {
		return "", fmt.Errorf("%w: .git is not accessible", domain.ErrInvalid)
	}
	// Check the deepest existing ancestor for symlink escapes.
	probe := p
	for {
		real, err := filepath.EvalSymlinks(probe)
		if err == nil {
			if real != dir && !strings.HasPrefix(real, dir+string(os.PathSeparator)) {
				return "", fmt.Errorf("%w: path escapes the project through a symlink", domain.ErrInvalid)
			}
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(probe)
		if parent == probe || len(parent) < len(dir) {
			break
		}
		probe = parent
	}
	return p, nil
}

func (w *Workspace) Init(ctx context.Context, projectID, template string) error {
	dir, err := w.projectDir(projectID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%w: workspace already exists", domain.ErrConflict)
	}
	src := filepath.Join(w.templates, filepath.Base(template))
	if err := copyTree(src, dir); err != nil {
		os.RemoveAll(dir)
		return fmt.Errorf("copy template: %w", err)
	}
	if _, err := w.git(ctx, dir, "init", "-q", "-b", "main"); err != nil {
		os.RemoveAll(dir)
		return err
	}
	if _, err := w.Commit(ctx, projectID, "project created from template "+template); err != nil {
		os.RemoveAll(dir)
		return err
	}
	return nil
}

func (w *Workspace) ReadFile(_ context.Context, projectID, path string) ([]byte, error) {
	p, err := w.resolve(projectID, path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", domain.ErrNotFound, path)
	}
	return data, err
}

func (w *Workspace) WriteFile(_ context.Context, projectID, path string, data []byte) error {
	p, err := w.resolve(projectID, path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func (w *Workspace) List(_ context.Context, projectID, dir string) ([]port.FileInfo, error) {
	base, err := w.resolve(projectID, dir)
	if err != nil {
		return nil, err
	}
	projectDir, _ := w.projectDir(projectID)
	out := []port.FileInfo{}
	err = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == base {
			return nil
		}
		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel(projectDir, p)
		info, err := d.Info()
		if err != nil {
			return err
		}
		out = append(out, port.FileInfo{Path: filepath.ToSlash(rel), IsDir: d.IsDir(), Size: info.Size()})
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", domain.ErrNotFound, dir)
	}
	return out, err
}

func (w *Workspace) Commit(ctx context.Context, projectID, message string) (string, error) {
	dir, err := w.projectDir(projectID)
	if err != nil {
		return "", err
	}
	if _, err := w.git(ctx, dir, "add", "-A"); err != nil {
		return "", err
	}
	// --allow-empty: every agent run is a commit, even one that only answered a question.
	if _, err := w.git(ctx, dir, "commit", "-q", "--allow-empty", "-m", message); err != nil {
		return "", err
	}
	out, err := w.git(ctx, dir, "rev-parse", "HEAD")
	return strings.TrimSpace(out), err
}

func (w *Workspace) Diff(ctx context.Context, projectID, from, to string) (string, error) {
	dir, err := w.projectDir(projectID)
	if err != nil {
		return "", err
	}
	for _, h := range []string{from, to} {
		if !hashPattern.MatchString(h) {
			return "", fmt.Errorf("%w: bad commit %q", domain.ErrInvalid, h)
		}
	}
	return w.git(ctx, dir, "diff", from, to)
}

// Revert restores the tree of hash as a new commit, so history is never rewritten.
func (w *Workspace) Revert(ctx context.Context, projectID, hash string) error {
	dir, err := w.projectDir(projectID)
	if err != nil {
		return err
	}
	if !hashPattern.MatchString(hash) {
		return fmt.Errorf("%w: bad commit %q", domain.ErrInvalid, hash)
	}
	if _, err := w.git(ctx, dir, "rm", "-r", "-q", "--ignore-unmatch", "."); err != nil {
		return err
	}
	if _, err := w.git(ctx, dir, "checkout", hash, "--", "."); err != nil {
		return err
	}
	_, err = w.Commit(ctx, projectID, "revert to "+hash)
	return err
}

// git runs git with explicit args (never a shell string) and a minimal identity.
func (w *Workspace) git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"), "HOME=" + dir,
		"GIT_AUTHOR_NAME=Game Forge", "GIT_AUTHOR_EMAIL=forge@localhost",
		"GIT_COMMITTER_NAME=Game Forge", "GIT_COMMITTER_EMAIL=forge@localhost",
		"GIT_CONFIG_NOSYSTEM=1",
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			if skipDirs[d.Name()] && p != src {
				return filepath.SkipDir
			}
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil // never copy symlinks or devices from the template
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
