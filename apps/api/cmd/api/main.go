package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/api"
	"github.com/belchiorjr/game-forge/apps/api/internal/adapter/eventbus"
	"github.com/belchiorjr/game-forge/apps/api/internal/adapter/fsworkspace"
	httpadapter "github.com/belchiorjr/game-forge/apps/api/internal/adapter/http"
	"github.com/belchiorjr/game-forge/apps/api/internal/adapter/llm"
	"github.com/belchiorjr/game-forge/apps/api/internal/adapter/sqlite"
	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/agent"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/cancelrun"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/createproject"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/getproject"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/getrun"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/listfiles"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/listprojects"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/listruns"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/llmsettings"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/readfile"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/startrun"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/streamevents"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/writefile"
)

type config struct {
	Addr, DataDir, WorkspacesDir, TemplatesDir, Template, Token string
	LLM, Model                                                  string
	Fallback                                                    bool
	FakeDelay, RunTimeout                                       time.Duration
	MaxSteps                                                    int
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func loadConfig() config {
	maxSteps, _ := strconv.Atoi(env("FORGE_MAX_STEPS", "60"))
	fakeDelay, _ := time.ParseDuration(env("FORGE_FAKE_DELAY", "700ms"))
	timeout, _ := time.ParseDuration(env("FORGE_RUN_TIMEOUT", "30m"))
	return config{
		Addr:          env("FORGE_ADDR", ":8080"),
		DataDir:       env("FORGE_DATA_DIR", "../../data"),
		WorkspacesDir: env("FORGE_WORKSPACES_DIR", "../../workspaces"),
		TemplatesDir:  env("FORGE_TEMPLATES_DIR", "../../templates"),
		Template:      env("FORGE_TEMPLATE", "ebiten-game"),
		Token:         os.Getenv("FORGE_API_TOKEN"),
		LLM:           env("FORGE_LLM", "fake"),
		Model:         env("FORGE_MODEL", "claude-opus-5-5"),
		Fallback:      env("FORGE_REFUSAL_FALLBACK", "true") == "true",
		FakeDelay:     fakeDelay,
		RunTimeout:    timeout,
		MaxSteps:      maxSteps,
	}
}

// loadSecret returns the key that encrypts stored settings: FORGE_SECRET_KEY, or a random key
// generated once and kept in the data directory.
func loadSecret(cfg config) ([]byte, error) {
	if v := os.Getenv("FORGE_SECRET_KEY"); v != "" {
		return []byte(v), nil
	}
	path := filepath.Join(cfg.DataDir, "secret.key")
	if b, err := os.ReadFile(path); err == nil && len(b) >= 32 {
		return b, nil
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	key := []byte(hex.EncodeToString(b))
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, err
	}
	slog.Info("generated settings encryption key", "path", path)
	return key, nil
}

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func must[T any](v T, err error) T {
	if err != nil {
		slog.Error("startup failed", "err", err)
		os.Exit(1)
	}
	return v
}

func main() {
	cfg := loadConfig()
	if cfg.Token == "" {
		slog.Warn("FORGE_API_TOKEN is empty: the API accepts unauthenticated requests (local use only)")
	}

	// Adapters.
	_ = os.MkdirAll(cfg.DataDir, 0o755)
	db := must(sqlite.Open(filepath.Join(cfg.DataDir, "forge.db")))
	defer db.Close()
	projects, runs := sqlite.NewProjectRepo(db), sqlite.NewRunRepo(db)
	ws := must(fsworkspace.New(cfg.WorkspacesDir, cfg.TemplatesDir))
	bus := eventbus.New(sqlite.NewEventRepo(db), time.Now)

	secret := must(loadSecret(cfg))
	settingsRepo := must(sqlite.NewSettingsRepo(db, secret))

	// The router reads the settings on every call, so keys saved in the web app apply to the next run.
	var settings *llmsettings.UseCase
	router := llm.NewRouter(func(ctx context.Context) (domain.LLMSettings, error) { return settings.Effective(ctx) }, llm.NewFake(cfg.FakeDelay))
	settings = llmsettings.New(settingsRepo, router, llmsettings.Defaults{
		Provider:        domain.LLMProvider(cfg.LLM),
		Model:           cfg.Model,
		RefusalFallback: cfg.Fallback,
		EnvAPIKey:       os.Getenv("ANTHROPIC_API_KEY"),
	}, time.Now)
	var model port.LLM = router

	if n, err := runs.FailOrphans(context.Background()); err == nil && n > 0 {
		slog.Warn("marked runs left over from a previous process as failed", "count", n)
	}

	// Use cases.
	agentCfg := agent.DefaultConfig()
	agentCfg.MaxSteps = cfg.MaxSteps
	loop := agent.NewLoop(projects, runs, ws, model, bus, time.Now, agentCfg)
	sup := agent.NewSupervisor(loop, cfg.RunTimeout)

	uc := httpadapter.UseCases{
		CreateProject: createproject.New(projects, ws, cfg.Template, time.Now, newID),
		ListProjects:  listprojects.New(projects),
		GetProject:    getproject.New(projects, runs),
		StartRun:      startrun.New(projects, runs, sup, time.Now, newID),
		CancelRun:     cancelrun.New(runs, sup),
		ListRuns:      listruns.New(projects, runs),
		GetRun:        getrun.New(runs),
		StreamEvents:  streamevents.New(projects, bus),
		ListFiles:     listfiles.New(projects, ws),
		ReadFile:      readfile.New(projects, ws),
		WriteFile:     writefile.New(projects, runs, ws, bus),
		LLMSettings:   settings,
	}
	// Cancelled on shutdown so long-lived SSE requests end instead of holding Shutdown open.
	baseCtx, cancelBase := context.WithCancel(context.Background())
	srv := &http.Server{
		Addr:              cfg.Addr,
		BaseContext:       func(net.Listener) context.Context { return baseCtx },
		Handler:           httpadapter.NewServer(uc, cfg.Token, api.OpenAPI).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("api listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cancelBase()
	_ = srv.Shutdown(ctx)
	sup.Shutdown() // active runs record "cancelled" before the DB closes
}
