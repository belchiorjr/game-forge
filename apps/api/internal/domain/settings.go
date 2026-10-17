package domain

import (
	"fmt"
	"strings"
	"time"
)

type LLMProvider string

const (
	ProviderFake      LLMProvider = "fake"
	ProviderAnthropic LLMProvider = "anthropic"
)

// LLMSettings is how the agent talks to a model. APIKey is a secret: it is stored encrypted
// and never leaves the API in clear text.
type LLMSettings struct {
	Provider        LLMProvider
	Model           string
	APIKey          string
	RefusalFallback bool
	UpdatedAt       time.Time
}

func (s *LLMSettings) Validate() error {
	s.Model = strings.TrimSpace(s.Model)
	s.APIKey = strings.TrimSpace(s.APIKey)
	switch s.Provider {
	case ProviderFake:
	case ProviderAnthropic:
		if s.Model == "" {
			return fmt.Errorf("%w: model is required for anthropic", ErrInvalid)
		}
	default:
		return fmt.Errorf("%w: provider must be fake or anthropic", ErrInvalid)
	}
	if len(s.APIKey) > 512 || strings.ContainsAny(s.APIKey, " \n\t") {
		return fmt.Errorf("%w: api key looks malformed", ErrInvalid)
	}
	return nil
}

// MaskSecret shows only the last 4 characters, enough to recognise which key is set.
func MaskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "••••"
	}
	return "••••" + s[len(s)-4:]
}
