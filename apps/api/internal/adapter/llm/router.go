package llm

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

// SettingsSource returns the configuration in effect right now.
type SettingsSource func(ctx context.Context) (domain.LLMSettings, error)

// Router implements port.LLM by picking the provider from the current settings on every call,
// so a key saved in the web app takes effect on the next run without restarting the API.
type Router struct {
	settings SettingsSource
	fake     *Fake

	mu     sync.Mutex
	cached *Anthropic
	sig    string
}

func NewRouter(settings SettingsSource, fake *Fake) *Router {
	return &Router{settings: settings, fake: fake}
}

func (r *Router) Complete(ctx context.Context, system string, msgs []port.Message, tools []port.ToolDef) (port.LLMResponse, error) {
	s, err := r.settings(ctx)
	if err != nil {
		return port.LLMResponse{}, err
	}
	if s.Provider != domain.ProviderAnthropic {
		return r.fake.Complete(ctx, system, msgs, tools)
	}
	if s.APIKey == "" {
		return port.LLMResponse{}, errors.New("no Anthropic API key configured: add one in Settings")
	}
	return r.anthropic(s).Complete(ctx, system, msgs, tools)
}

func (r *Router) anthropic(s domain.LLMSettings) *Anthropic {
	sig := fmt.Sprintf("%s|%s|%t", s.APIKey, s.Model, s.RefusalFallback)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cached == nil || r.sig != sig {
		r.cached, r.sig = NewAnthropic(s.Model, s.RefusalFallback, option.WithAPIKey(s.APIKey)), sig
	}
	return r.cached
}

// Test implements port.LLMTester with a model lookup, which checks key and model without
// spending tokens.
func (r *Router) Test(ctx context.Context, s domain.LLMSettings) error {
	if s.Provider != domain.ProviderAnthropic {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	client := anthropic.NewClient(option.WithAPIKey(s.APIKey), option.WithMaxRetries(0))
	_, err := client.Models.Get(ctx, s.Model, anthropic.ModelGetParams{})
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case 401, 403:
			return fmt.Errorf("%w: invalid API key", domain.ErrInvalid)
		case 404:
			return fmt.Errorf("%w: model %q not found", domain.ErrInvalid, s.Model)
		}
		return fmt.Errorf("anthropic returned %d", apiErr.StatusCode)
	}
	if err != nil {
		return fmt.Errorf("cannot reach Anthropic: %w", err)
	}
	return nil
}
