package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type apiError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

// writeError maps domain errors to the single error shape of the contract.
// A "code:" right after the sentinel (e.g. "conflict: run_active: ...") refines the code.
func writeError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "internal"
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, domain.ErrConflict):
		status, code = http.StatusConflict, "conflict"
	case errors.Is(err, domain.ErrInvalid):
		status, code = http.StatusUnprocessableEntity, "invalid"
	case errors.Is(err, domain.ErrLimitExceeded):
		status, code = http.StatusTooManyRequests, "limit_exceeded"
	}
	msg := "internal error"
	if status == http.StatusInternalServerError {
		slog.Error("request failed", "err", err)
	} else {
		msg = err.Error()
		parts := strings.SplitN(msg, ": ", 3)
		if len(parts) == 3 && !strings.Contains(parts[1], " ") {
			code, msg = parts[1], parts[2]
		} else if len(parts) >= 2 {
			msg = strings.Join(parts[1:], ": ")
		}
	}
	writeJSON(w, status, map[string]apiError{"error": {Code: code, Message: msg, Details: map[string]any{}}})
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	if err := dec.Decode(v); err != nil {
		return errors.Join(domain.ErrInvalid, errors.New("invalid JSON body"))
	}
	return nil
}
