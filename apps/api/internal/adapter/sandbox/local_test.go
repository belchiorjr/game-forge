package sandbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

func setup(t *testing.T, files map[string]string) *Local {
	t.Helper()
	root := t.TempDir()
	for p, c := range files {
		full := filepath.Join(root, "p1", p)
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(c), 0o644)
	}
	cache := t.TempDir()
	l, err := NewLocal(root, filepath.Join(cache, "build"), filepath.Join(cache, "mod"))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestRunsAllowedCommandWithScrubbedEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-should-not-leak")
	l := setup(t, map[string]string{
		"go.mod":       "module x\n\ngo 1.25\n",
		"main_test.go": "package main\nimport (\"os\";\"testing\")\nfunc TestEnv(t *testing.T){ if os.Getenv(\"ANTHROPIC_API_KEY\") != \"\" { t.Fatal(\"leak\") } }\n",
	})
	res, err := l.Run(context.Background(), port.CommandSpec{ProjectID: "p1", Name: "go", Args: []string{"test", "./..."}, Timeout: time.Minute})
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("res = %+v err = %v", res, err)
	}
}

func TestRejectsDisallowed(t *testing.T) {
	l := setup(t, nil)
	for _, spec := range []port.CommandSpec{
		{ProjectID: "p1", Name: "sh", Args: []string{"-c", "id"}},
		{ProjectID: "p1", Name: "go", Args: []string{"run", "."}},
		{ProjectID: "p1", Name: "go", Args: []string{"build", "-toolexec=evil"}},
		{ProjectID: "p1", Name: "go", Args: []string{"build", "../other"}},
		{ProjectID: "../p1", Name: "go", Args: []string{"build"}},
		{ProjectID: "p1", Name: "go", Args: []string{"build"}, Env: map[string]string{"LD_PRELOAD": "x"}},
	} {
		if _, err := l.Run(context.Background(), spec); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%v %v: want ErrInvalid, got %v", spec.Name, spec.Args, err)
		}
	}
}

func TestTimeoutKillsProcess(t *testing.T) {
	l := setup(t, map[string]string{
		"go.mod":       "module x\n\ngo 1.25\n",
		"main_test.go": "package main\nimport (\"testing\";\"time\")\nfunc TestHang(t *testing.T){ time.Sleep(time.Hour) }\n",
	})
	start := time.Now()
	res, err := l.Run(context.Background(), port.CommandSpec{ProjectID: "p1", Name: "go", Args: []string{"test", "./..."}, Timeout: 3 * time.Second})
	if err != nil || !res.TimedOut || time.Since(start) > 20*time.Second {
		t.Fatalf("res = %+v err = %v after %v", res, err, time.Since(start))
	}
}

func TestCapOutput(t *testing.T) {
	out := capOutput(strings.Repeat("a", 100)+strings.Repeat("z", 100), 20)
	if !strings.HasPrefix(out, "aaaaaaaaaa") || !strings.HasSuffix(out, "zzzzzzzzzz") {
		t.Fatalf("out = %q", out)
	}
}
