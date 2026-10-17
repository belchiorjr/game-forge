// Package llmsettings reads, updates and tests the agent's model configuration.
package llmsettings

import (
	"context"
	"fmt"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

// View is what callers may see: the key is masked, and Source says where it comes from.
type View struct {
	Provider        domain.LLMProvider
	Model           string
	RefusalFallback bool
	APIKeySet       bool
	APIKeyMasked    string
	APIKeySource    string // "settings", "env" or ""
	UpdatedAt       *time.Time
}

type Update struct {
	Provider        string
	Model           string
	APIKey          *string // nil keeps the stored key, "" removes it
	RefusalFallback bool
}

// Defaults apply when nothing was saved; EnvAPIKey is the server's ANTHROPIC_API_KEY.
type Defaults struct {
	Provider        domain.LLMProvider
	Model           string
	RefusalFallback bool
	EnvAPIKey       string
}

type UseCase struct {
	repo     port.SettingsRepository
	tester   port.LLMTester
	defaults Defaults
	clock    func() time.Time
}

func New(repo port.SettingsRepository, tester port.LLMTester, defaults Defaults, clock func() time.Time) *UseCase {
	return &UseCase{repo: repo, tester: tester, defaults: defaults, clock: clock}
}

// Effective returns the configuration the agent should use right now, key included.
func (uc *UseCase) Effective(ctx context.Context) (domain.LLMSettings, error) {
	s, err := uc.repo.GetLLM(ctx)
	if err != nil {
		return domain.LLMSettings{}, err
	}
	if s == nil {
		s = &domain.LLMSettings{Provider: uc.defaults.Provider, Model: uc.defaults.Model, RefusalFallback: uc.defaults.RefusalFallback}
	}
	if s.APIKey == "" {
		s.APIKey = uc.defaults.EnvAPIKey
	}
	return *s, nil
}

func (uc *UseCase) Get(ctx context.Context) (View, error) {
	stored, err := uc.repo.GetLLM(ctx)
	if err != nil {
		return View{}, err
	}
	eff, err := uc.Effective(ctx)
	if err != nil {
		return View{}, err
	}
	v := View{Provider: eff.Provider, Model: eff.Model, RefusalFallback: eff.RefusalFallback, APIKeySet: eff.APIKey != "", APIKeyMasked: domain.MaskSecret(eff.APIKey)}
	switch {
	case stored != nil && stored.APIKey != "":
		v.APIKeySource = "settings"
	case eff.APIKey != "":
		v.APIKeySource = "env"
	}
	if stored != nil {
		t := stored.UpdatedAt
		v.UpdatedAt = &t
	}
	return v, nil
}

func (uc *UseCase) Update(ctx context.Context, in Update) (View, error) {
	stored, err := uc.repo.GetLLM(ctx)
	if err != nil {
		return View{}, err
	}
	next := domain.LLMSettings{Provider: domain.LLMProvider(in.Provider), Model: in.Model, RefusalFallback: in.RefusalFallback, UpdatedAt: uc.clock()}
	switch {
	case in.APIKey != nil:
		next.APIKey = *in.APIKey
	case stored != nil:
		next.APIKey = stored.APIKey
	}
	if err := next.Validate(); err != nil {
		return View{}, err
	}
	if err := uc.repo.SaveLLM(ctx, &next); err != nil {
		return View{}, err
	}
	return uc.Get(ctx)
}

// Test checks the given configuration, or the effective one when in is nil.
// A nil APIKey in the update means "use the stored key", so the browser never needs it back.
func (uc *UseCase) Test(ctx context.Context, in *Update) error {
	s, err := uc.Effective(ctx)
	if err != nil {
		return err
	}
	if in != nil {
		s.Provider, s.Model, s.RefusalFallback = domain.LLMProvider(in.Provider), in.Model, in.RefusalFallback
		if in.APIKey != nil && *in.APIKey != "" {
			s.APIKey = *in.APIKey
		}
	}
	if err := s.Validate(); err != nil {
		return err
	}
	if s.Provider == domain.ProviderAnthropic && s.APIKey == "" {
		return fmt.Errorf("%w: no API key configured", domain.ErrInvalid)
	}
	return uc.tester.Test(ctx, s)
}
