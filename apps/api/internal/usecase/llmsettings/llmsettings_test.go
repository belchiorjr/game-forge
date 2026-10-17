package llmsettings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
)

type memRepo struct{ s *domain.LLMSettings }

func (m *memRepo) GetLLM(context.Context) (*domain.LLMSettings, error) {
	if m.s == nil {
		return nil, nil
	}
	c := *m.s
	return &c, nil
}
func (m *memRepo) SaveLLM(_ context.Context, s *domain.LLMSettings) error {
	c := *s
	m.s = &c
	return nil
}

type recTester struct{ got domain.LLMSettings }

func (r *recTester) Test(_ context.Context, s domain.LLMSettings) error { r.got = s; return nil }

func ptr(s string) *string { return &s }

func newUC(env string) (*UseCase, *memRepo, *recTester) {
	repo, tester := &memRepo{}, &recTester{}
	d := Defaults{Provider: domain.ProviderFake, Model: "claude-opus-5-5", RefusalFallback: true, EnvAPIKey: env}
	return New(repo, tester, d, func() time.Time { return time.Unix(0, 0) }), repo, tester
}

func TestDefaultsAndEnvKey(t *testing.T) {
	uc, _, _ := newUC("sk-ant-env-1234")
	v, err := uc.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.Provider != domain.ProviderFake || v.APIKeySource != "env" || v.APIKeyMasked != "••••1234" {
		t.Fatalf("view = %+v", v)
	}
}

func TestUpdateKeepsKeyWhenOmitted(t *testing.T) {
	ctx := context.Background()
	uc, repo, _ := newUC("")
	if _, err := uc.Update(ctx, Update{Provider: "anthropic", Model: "claude-opus-5-5", APIKey: ptr("sk-ant-secret-abcd")}); err != nil {
		t.Fatal(err)
	}
	v, err := uc.Update(ctx, Update{Provider: "anthropic", Model: "claude-sonnet-5-5"})
	if err != nil {
		t.Fatal(err)
	}
	if repo.s.APIKey != "sk-ant-secret-abcd" || v.APIKeySource != "settings" || v.Model != "claude-sonnet-5-5" {
		t.Fatalf("repo = %+v view = %+v", repo.s, v)
	}
	if _, err := uc.Update(ctx, Update{Provider: "anthropic", Model: "claude-opus-5-5", APIKey: ptr("")}); err != nil {
		t.Fatal(err)
	}
	if repo.s.APIKey != "" {
		t.Fatal("empty key should remove it")
	}
}

func TestTestUsesStoredKeyAndRejectsMissing(t *testing.T) {
	ctx := context.Background()
	uc, _, tester := newUC("")
	err := uc.Test(ctx, &Update{Provider: "anthropic", Model: "claude-opus-5-5"})
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("want ErrInvalid without key, got %v", err)
	}
	_, _ = uc.Update(ctx, Update{Provider: "anthropic", Model: "claude-opus-5-5", APIKey: ptr("sk-ant-stored-9999")})
	if err := uc.Test(ctx, &Update{Provider: "anthropic", Model: "claude-opus-5-5"}); err != nil {
		t.Fatal(err)
	}
	if tester.got.APIKey != "sk-ant-stored-9999" {
		t.Fatalf("tester got %+v", tester.got)
	}
}
