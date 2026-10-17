// Package http exposes the use cases over REST + SSE (contract: api/openapi.yaml).
package http

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/cancelrun"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/createproject"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/getproject"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/getrun"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/listfiles"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/listprojects"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/listruns"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/readfile"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/startrun"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/streamevents"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/writefile"
)

type UseCases struct {
	CreateProject *createproject.UseCase
	ListProjects  *listprojects.UseCase
	GetProject    *getproject.UseCase
	StartRun      *startrun.UseCase
	CancelRun     *cancelrun.UseCase
	ListRuns      *listruns.UseCase
	GetRun        *getrun.UseCase
	StreamEvents  *streamevents.UseCase
	ListFiles     *listfiles.UseCase
	ReadFile      *readfile.UseCase
	WriteFile     *writefile.UseCase
}

type Server struct {
	uc        UseCases
	token     string
	openapi   []byte
	heartbeat time.Duration
	idem      *idempotency
}

func NewServer(uc UseCases, token string, openapi []byte) *Server {
	return &Server{uc: uc, token: token, openapi: openapi, heartbeat: 15 * time.Second, idem: newIdempotency(10 * time.Minute)}
}

func (s *Server) Handler() http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("POST /projects", s.createProject)
	api.HandleFunc("GET /projects", s.listProjects)
	api.HandleFunc("GET /projects/{id}", s.getProject)
	api.HandleFunc("POST /projects/{id}/runs", s.startRun)
	api.HandleFunc("GET /projects/{id}/runs", s.listRuns)
	api.HandleFunc("GET /projects/{id}/runs/{run_id}", s.getRun)
	api.HandleFunc("POST /projects/{id}/runs/{run_id}/cancel", s.cancelRun)
	api.HandleFunc("GET /projects/{id}/events", s.events)
	api.HandleFunc("GET /projects/{id}/files", s.listFiles)
	api.HandleFunc("GET /projects/{id}/files/content", s.readFile)
	api.HandleFunc("PUT /projects/{id}/files/content", s.writeFile)

	root := http.NewServeMux()
	root.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	root.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(s.openapi)
	})
	root.Handle("/api/v1/", http.StripPrefix("/api/v1", s.auth(api)))
	return logRequests(root)
}

// auth is the seam for real authentication; the MVP uses one bearer token.
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token != "" {
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
				writeJSON(w, http.StatusUnauthorized, map[string]apiError{"error": {Code: "unauthorized", Message: "missing or invalid token", Details: map[string]any{}}})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start).Round(time.Millisecond))
	})
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name        string `json:"name"`
		Idea        string `json:"idea"`
		Genre       string `json:"genre"`
		Orientation string `json:"orientation"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	out, err := s.uc.CreateProject.Execute(r.Context(), createproject.Input{Name: in.Name, Idea: in.Idea, Genre: in.Genre, Orientation: in.Orientation})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toProject(out.Project))
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	ps, err := s.uc.ListProjects.Execute(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]projectDTO, len(ps))
	for i, p := range ps {
		out[i] = toProject(p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": out})
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	out, err := s.uc.GetProject.Execute(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	dto := toProject(out.Project)
	dto.LatestRun = toRun(out.LatestRun)
	writeJSON(w, http.StatusOK, dto)
}

func (s *Server) startRun(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Prompt string `json:"prompt"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	projectID := r.PathValue("id")
	key := r.Header.Get("Idempotency-Key")
	if runID, ok := s.idem.get(projectID, key); ok {
		writeJSON(w, http.StatusAccepted, map[string]string{"run_id": runID})
		return
	}
	out, err := s.uc.StartRun.Execute(r.Context(), startrun.Input{ProjectID: projectID, Prompt: in.Prompt})
	if err != nil {
		writeError(w, err)
		return
	}
	s.idem.put(projectID, key, out.RunID)
	writeJSON(w, http.StatusAccepted, map[string]string{"run_id": out.RunID})
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := s.uc.ListRuns.Execute(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]*runDTO, len(runs))
	for i, run := range runs {
		out[i] = toRun(run)
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": out})
}

func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	run, err := s.uc.GetRun.Execute(r.Context(), r.PathValue("id"), r.PathValue("run_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toRun(run))
}

func (s *Server) cancelRun(w http.ResponseWriter, r *http.Request) {
	err := s.uc.CancelRun.Execute(r.Context(), cancelrun.Input{ProjectID: r.PathValue("id"), RunID: r.PathValue("run_id")})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "cancelling"})
}

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	files, err := s.uc.ListFiles.Execute(r.Context(), r.PathValue("id"), r.URL.Query().Get("dir"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": toFiles(files)})
}

func (s *Server) readFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	data, err := s.uc.ReadFile.Execute(r.Context(), r.PathValue("id"), path)
	if err != nil {
		writeError(w, err)
		return
	}
	ct := http.DetectContentType(data)
	if strings.HasPrefix(ct, "text/plain") {
		ct = "text/plain; charset=utf-8"
	}
	w.Header().Set("Content-Type", ct)
	_, _ = w.Write(data)
}

func (s *Server) writeFile(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, (1<<20)+1))
	if err != nil {
		writeError(w, fmt.Errorf("%w: body too large", domain.ErrInvalid))
		return
	}
	out, err := s.uc.WriteFile.Execute(r.Context(), writefile.Input{ProjectID: r.PathValue("id"), Path: r.URL.Query().Get("path"), Content: data})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"commit": out.Commit})
}

// events streams the project timeline as SSE, replaying after Last-Event-ID.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, fmt.Errorf("streaming unsupported"))
		return
	}
	last, _ := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64)
	if q := r.URL.Query().Get("after"); q != "" {
		last, _ = strconv.ParseInt(q, 10, 64)
	}
	ch, err := s.uc.StreamEvents.Execute(r.Context(), r.PathValue("id"), last)
	if err != nil {
		writeError(w, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "retry: 2000\n\n")
	flusher.Flush()

	ping := time.NewTicker(s.heartbeat)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case e, ok := <-ch:
			if !ok {
				return // dropped as a slow subscriber; the client reconnects and replays
			}
			writeEvent(w, e)
			flusher.Flush()
		}
	}
}

func writeEvent(w io.Writer, e domain.Event) {
	// data carries the payload plus the common envelope fields of the contract.
	payload := map[string]any{}
	if err := json.Unmarshal(e.Data, &payload); err != nil {
		payload = map[string]any{"value": json.RawMessage(e.Data)}
	}
	payload["seq"], payload["run_id"], payload["ts"] = e.Seq, e.RunID, e.TS.UTC().Format(time.RFC3339Nano)
	data, _ := json.Marshal(payload)
	fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.Seq, e.Type, data)
}
