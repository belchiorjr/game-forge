package http

import (
	"net/http"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/llmsettings"
)

// llmSettingsDTO never carries the key back: only whether one is set and its last 4 chars.
type llmSettingsDTO struct {
	Provider        string     `json:"provider"`
	Model           string     `json:"model"`
	RefusalFallback bool       `json:"refusal_fallback"`
	APIKeySet       bool       `json:"api_key_set"`
	APIKeyMasked    string     `json:"api_key_masked"`
	APIKeySource    string     `json:"api_key_source"`
	UpdatedAt       *time.Time `json:"updated_at"`
}

type llmSettingsInput struct {
	Provider        string  `json:"provider"`
	Model           string  `json:"model"`
	APIKey          *string `json:"api_key"` // omitted keeps the stored key, "" removes it
	RefusalFallback bool    `json:"refusal_fallback"`
}

func (in llmSettingsInput) update() llmsettings.Update {
	return llmsettings.Update{Provider: in.Provider, Model: in.Model, APIKey: in.APIKey, RefusalFallback: in.RefusalFallback}
}

func toLLMSettings(v llmsettings.View) llmSettingsDTO {
	return llmSettingsDTO{Provider: string(v.Provider), Model: v.Model, RefusalFallback: v.RefusalFallback,
		APIKeySet: v.APIKeySet, APIKeyMasked: v.APIKeyMasked, APIKeySource: v.APIKeySource, UpdatedAt: v.UpdatedAt}
}

func (s *Server) getLLMSettings(w http.ResponseWriter, r *http.Request) {
	v, err := s.uc.LLMSettings.Get(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toLLMSettings(v))
}

func (s *Server) putLLMSettings(w http.ResponseWriter, r *http.Request) {
	var in llmSettingsInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	v, err := s.uc.LLMSettings.Update(r.Context(), in.update())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toLLMSettings(v))
}

// testLLMSettings checks the posted (unsaved) values, falling back to the stored key.
func (s *Server) testLLMSettings(w http.ResponseWriter, r *http.Request) {
	var in llmSettingsInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	u := in.update()
	if err := s.uc.LLMSettings.Test(r.Context(), &u); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": errMessage(err)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Conexão OK"})
}
