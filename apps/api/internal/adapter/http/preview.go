package http

import (
	"errors"
	"net/http"
	"os"
	"path"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
)

var previewTypes = map[string]string{
	".html": "text/html; charset=utf-8",
	".js":   "text/javascript; charset=utf-8",
	".wasm": "application/wasm", // required for streaming compilation
	".png":  "image/png",
	".json": "application/json",
}

// preview serves the project's web/ directory. wasm_exec.js comes from the server's own Go
// toolchain, which is the one that compiled game.wasm, so the two always match.
func (s *Server) preview(w http.ResponseWriter, r *http.Request) {
	file := path.Clean("/" + r.PathValue("file"))[1:]
	if file == "" {
		file = "index.html"
	}
	ct, ok := previewTypes[path.Ext(file)]
	if !ok {
		writeError(w, domain.ErrNotFound)
		return
	}
	var data []byte
	var err error
	if file == "wasm_exec.js" && s.wasmExec != "" {
		data, err = os.ReadFile(s.wasmExec)
	} else {
		data, err = s.uc.ReadFile.Execute(r.Context(), r.PathValue("id"), "web/"+file)
	}
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) && file == "game.wasm" {
			writeJSON(w, http.StatusNotFound, map[string]apiError{"error": {Code: "no_build", Message: "no preview build yet", Details: map[string]any{}}})
			return
		}
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

func (s *Server) startBuild(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Target string `json:"target"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	if in.Target != "wasm" {
		writeError(w, errors.Join(domain.ErrInvalid, errors.New("only the wasm target is available")))
		return
	}
	if err := s.uc.BuildPreview.Execute(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "building"})
}
