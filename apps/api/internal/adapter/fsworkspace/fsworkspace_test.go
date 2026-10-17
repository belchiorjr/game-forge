package fsworkspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
)

func setup(t *testing.T) *Workspace {
	t.Helper()
	tmp := t.TempDir()
	tpl := filepath.Join(tmp, "templates", "basic")
	os.MkdirAll(filepath.Join(tpl, "docs"), 0o755)
	os.WriteFile(filepath.Join(tpl, "docs", "GDD.md"), []byte("# GDD\n"), 0o644)
	w, err := New(filepath.Join(tmp, "ws"), filepath.Join(tmp, "templates"))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Init(context.Background(), "p1", "basic"); err != nil {
		t.Fatal(err)
	}
	return w
}

func TestReadWriteCommitRevert(t *testing.T) {
	ctx := context.Background()
	w := setup(t)
	first, err := w.Commit(ctx, "p1", "noop")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteFile(ctx, "p1", "internal/logic/a.go", []byte("package logic\n")); err != nil {
		t.Fatal(err)
	}
	second, err := w.Commit(ctx, "p1", "add logic")
	if err != nil {
		t.Fatal(err)
	}
	diff, err := w.Diff(ctx, "p1", first, second)
	if err != nil || !strings.Contains(diff, "internal/logic/a.go") {
		t.Fatalf("diff = %q, %v", diff, err)
	}
	if err := w.Revert(ctx, "p1", first); err != nil {
		t.Fatal(err)
	}
	if _, err := w.ReadFile(ctx, "p1", "internal/logic/a.go"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("file should be gone after revert, got %v", err)
	}
	if got, _ := w.ReadFile(ctx, "p1", "docs/GDD.md"); string(got) != "# GDD\n" {
		t.Fatalf("GDD = %q", got)
	}
}

func TestPathJail(t *testing.T) {
	ctx := context.Background()
	w := setup(t)
	outside := t.TempDir()
	os.Symlink(outside, filepath.Join(w.root, "p1", "escape"))

	for _, p := range []string{"/etc/passwd", ".git/config", "escape/x.txt", "../../x.txt", "docs/../../x.txt"} {
		if err := w.WriteFile(ctx, "p1", p, []byte("x")); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("write %q: want ErrInvalid, got %v", p, err)
		}
	}
	if err := w.WriteFile(ctx, "p1", "docs/../ok.txt", []byte("x")); err != nil {
		t.Fatalf("inner .. should be allowed: %v", err)
	}
	if _, err := w.ReadFile(ctx, "../p1", "docs/GDD.md"); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("bad project id: want ErrInvalid, got %v", err)
	}
}
